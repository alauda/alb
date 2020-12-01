package controller

import (
	"crypto/md5"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"k8s.io/apimachinery/pkg/types"
	"math"
	"math/rand"
	"os"
	"path"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/thoas/go-funk"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"

	"alauda.io/alb2/config"
	"alauda.io/alb2/driver"
	"alauda.io/alb2/utils"
)

var (
	SUCCESS              = "success"
	FAILED               = "failed"
	StatusFileParentPath = "/var/run/alb/last_status"
)

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}

func getServiceName(id string, port int) string {
	if port != 0 {
		return fmt.Sprintf("%s-%d", id, port)
	}
	return id
}

func generateBackend(serviceMap map[string][]*driver.Backend, services []*BackendService) []*Backend {
	totalWeight := 0
	for _, svc := range services {
		if svc.Weight > 100 {
			svc.Weight = 100
		}
		if svc.Weight < 0 {
			svc.Weight = 0
		}
		totalWeight += svc.Weight
	}
	if totalWeight == 0 {
		// all service has zero weight
		totalWeight = 100
	}
	bes := []*Backend{}
	for _, svc := range services {
		name := getServiceName(svc.ServiceID, svc.ContainerPort)
		backends, ok := serviceMap[name]
		if !ok || len(backends) == 0 {
			name = getServiceName(svc.ServiceID, 0)
			backends, ok = serviceMap[name]
			if !ok || len(backends) == 0 {
				continue
			}
		}
		//100 is the max weigh in SLB
		weight := int(math.Floor(float64(svc.Weight*100)/float64(totalWeight*len(backends)) + 0.5))
		if weight == 0 && svc.Weight != 0 {
			weight = 1
		}
		for _, be := range backends {
			port := be.Port
			if port == 0 {
				port = svc.ContainerPort
			}
			bes = append(bes,
				&Backend{
					Address: be.IP,
					Port:    port,
					Weight:  weight,
				})
		}
	}
	return bes
}

func merge(loadBalancers []*LoadBalancer, services []*driver.Service) {
	serviceMap := make(map[string][]*driver.Backend)
	for _, svc := range services {
		if svc == nil {
			continue
		}
		if svc.ServicePort == 0 {
			svc.ServicePort = svc.ContainerPort
		}
		name := getServiceName(svc.ServiceID, svc.ServicePort)
		serviceMap[name] = svc.Backends
	}
	for _, lb := range loadBalancers {
		// lb.Frontends = make(map[]*Frontend)
		for _, ft := range lb.Frontends {
			var rules RuleList
			for _, rule := range ft.Rules {
				if len(rule.Services) == 0 {
					klog.Warningf("rule %s has no active service.", rule.RuleID)
				}
				rule.BackendGroup = &BackendGroup{
					Name: rule.RuleID,
					// bg.mode dont care whether http or https
					Mode:                     ModeHTTP,
					SessionAffinityPolicy:    rule.SessionAffinityPolicy,
					SessionAffinityAttribute: rule.SessionAffinityAttr,
				}
				rule.BackendGroup.Backends = generateBackend(serviceMap, rule.Services)
				rules = append(rules, rule)
			}
			if len(rules) > 0 {
				sort.Sort(rules)
				ft.Rules = rules
			} else {
				ft.Rules = RuleList{}
			}

			if len(ft.Services) == 0 {
				klog.Warningf("frontend %s has no default service.",
					ft.String())
			} else {
				ft.BackendGroup.Backends = generateBackend(serviceMap, ft.Services)
				if ft.Protocol == ProtocolTCP {
					ft.BackendGroup.Mode = ModeTCP
				} else {
					ft.BackendGroup.Mode = ModeHTTP
				}
			}
		}
	}
}

var cfgLocker sync.Mutex

func generateConfig(loadbalancer *LoadBalancer, driver *driver.KubernetesDriver) Config {
	cfgLocker.Lock()
	defer cfgLocker.Unlock()
	result := Config{
		Name:             loadbalancer.Name,
		Address:          loadbalancer.Address,
		BindAddress:      loadbalancer.BindAddress,
		LoadBalancerID:   loadbalancer.LoadBalancerID,
		Frontends:        make(map[int]*Frontend),
		BackendGroup:     []*BackendGroup{},
		CertificateMap:   make(map[string]Certificate),
		TweakHash:        loadbalancer.TweakHash,
		EnablePrometheus: config.Get("ENABLE_PROMETHEUS") == "true",
		EnableIPV6:       checkIPV6(),
		EnableHTTP2:      config.Get("ENABLE_HTTP2") == "true",
		CPUNum:           strconv.Itoa(utils.NumCPU(workerLimit())),
		MetricsPort:      config.GetInt("METRICS_PORT"),
	}
	var listenTCPPorts []int
	var err error
	if config.Get("ENABLE_PORTPROBE") == "true" {
		listenTCPPorts, err = utils.GetListenTCPPorts()
		if err != nil {
			klog.Error(err)
		}
		klog.V(2).Info("finish port probe, listen tcp ports: ", listenTCPPorts)
	}
	var portInfo map[string][]string
	if GetAlbRoleType(loadbalancer.Labels) == RolePort {
		portInfo, err = getPortInfo(driver)
		if err != nil {
			klog.Errorf("get port project info failed, %v", err)
		}
	}
	for _, ft := range loadbalancer.Frontends {
		conflict := false
		for _, port := range listenTCPPorts {
			if ft.Port == port {
				conflict = true
				klog.Errorf("skip port: %d has conflict", ft.Port)
				break
			}
		}
		if config.Get("ENABLE_PORTPROBE") == "true" {
			if err := driver.UpdateFrontendStatus(ft.RawName, conflict); err != nil {
				klog.Error(err)
			}
			if conflict {
				// skip conflict port
				continue
			}
		}
		if GetAlbRoleType(loadbalancer.Labels) == RolePort && portInfo != nil {
			// current projects
			portProjects := GetOwnProjects(ft.RawName, ft.Labels)
			// desired projects
			desiredPortProjects, err := getPortProject(ft.Port, portInfo)
			if err != nil {
				klog.Errorf("get port %d desired projects failed, %v", ft.Port, err)
				goto OUT
			}
			if diff := funk.Subtract(portProjects, desiredPortProjects); diff != nil {
				// diff need update
				payload := generatePatchPortProjectPayload(ft.Labels, desiredPortProjects)
				if _, err := driver.ALBClient.CrdV1().Frontends(config.Get("NAMESPACE")).Patch(ft.RawName, types.JSONPatchType, payload); err != nil {
					klog.Errorf("patch port %s project failed, %v", ft.RawName, err)
				}
			}
		}
	OUT:
		klog.Infof("generate config for ft %d %s, have %d rules", ft.Port, ft.Protocol, len(ft.Rules))
		isHTTP := ft.Protocol == ProtocolHTTP
		isHTTPS := ft.Protocol == ProtocolHTTPS
		if isHTTP || isHTTPS {
			if isHTTPS && ft.CertificateName != "" {
				secretNs, secretName, err := ParseCertificateName(ft.CertificateName)
				if err != nil {
					klog.Errorf("invalid certificateName, %s", ft.CertificateName)
					continue
				}
				cert, err := getCertificate(driver, secretNs, secretName)
				if err != nil {
					klog.Warningf("get cert %s failed, %+v", ft.CertificateName, err)
				} else {
					// default cert for port ft.Port
					result.CertificateMap[strconv.Itoa(ft.Port)] = *cert
				}
			}
			for _, rule := range ft.Rules {
				if isHTTPS && rule.Domain != "" && rule.CertificateName != "" {
					secretNs, secretName, err := ParseCertificateName(rule.CertificateName)
					if err != nil {
						klog.Errorf("invalid certificateName, %s", rule.CertificateName)
						continue
					}
					cert, err := getCertificate(driver, secretNs, secretName)
					if err != nil {
						klog.Warningf("get cert %s failed, %+v", rule.CertificateName, err)
						continue
					}
					if existCert, ok := result.CertificateMap[strings.ToLower(rule.Domain)]; ok {
						if existCert.Cert != cert.Cert || existCert.Key != cert.Key {
							klog.Warningf("declare different cert for host %s", strings.ToLower(rule.Domain))
							continue
						}
					}
					result.CertificateMap[strings.ToLower(rule.Domain)] = *cert
				}
				rule.Domain = strings.ToLower(rule.Domain)
				result.BackendGroup = append(result.BackendGroup, rule.BackendGroup)
			}
		}
		if ft.BackendGroup != nil && len(ft.BackendGroup.Backends) > 0 {
			// FIX: http://jira.alaudatech.com/browse/DEV-16954
			// remove duplicate upstream
			if !funk.Contains(result.BackendGroup, ft.BackendGroup) {
				result.BackendGroup = append(result.BackendGroup, ft.BackendGroup)
			}
		}

		result.Frontends[ft.Port] = ft
		sort.Sort(result.BackendGroup)
	} // end of  _, ft := range loadbalancer.Frontends

	return result
}

func sameFiles(file1, file2 string) bool {
	sum1, err := fileMd5(file1)
	if err != nil {
		klog.Error(err.Error())
		return false
	}
	sum2, err := fileMd5(file2)
	if err != nil {
		klog.Error(err.Error())
		return false
	}

	return sum1 == sum2
}

func fileMd5(file string) (string, error) {
	f, err := os.Open(file)
	if err != nil {
		klog.Error(err.Error())
		return "", err
	}
	md5h := md5.New()
	io.Copy(md5h, f)
	return fmt.Sprintf("%x", md5h.Sum(nil)), nil
}

func reverseStatus(status string) string {
	if status == SUCCESS {
		return FAILED
	}
	return SUCCESS
}

func setLastReloadStatus(status, statusFileParentPath string) error {
	statusFilePath := path.Join(statusFileParentPath, status)
	if _, err := os.Stat(statusFilePath); os.IsNotExist(err) {
		f, err := os.Create(statusFilePath)
		if err != nil {
			klog.Errorf("create new status file failed %s", err.Error())
			return err
		}
		f.Close()
	}

	reversStatusFilePath := path.Join(statusFileParentPath, reverseStatus(status))
	if _, err := os.Stat(reversStatusFilePath); err == nil {
		err := os.Remove(reversStatusFilePath)
		if err != nil {
			klog.Errorf("remove old status file failed %s", err.Error())
			return err
		}
	}
	return nil
}

func getLastReloadStatus(statusFileParentPath string) string {
	successStatusFilePath := path.Join(statusFileParentPath, SUCCESS)
	if _, err := os.Stat(successStatusFilePath); err == nil {
		klog.Infof("last reload status: %s", SUCCESS)
		return SUCCESS
	}
	klog.Info("last reload status", FAILED)
	return FAILED
}

func jsonEqual(a, b []byte) bool {
	var j, j2 interface{}
	if err := json.Unmarshal(a, &j); err != nil {
		return false
	}
	if err := json.Unmarshal(b, &j2); err != nil {
		return false
	}
	return reflect.DeepEqual(j2, j)
}

const ALPHANUM = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

func GetMD5Hash(text string) string {
	hasher := md5.New()
	hasher.Write([]byte(text))
	return hex.EncodeToString(hasher.Sum(nil))
}
func RandomStr(pixff string, length int) string {
	result := make([]byte, length)
	for i := 0; i < length; i++ {
		result[i] = ALPHANUM[rand.Intn(len(ALPHANUM))]
	}
	if pixff != "" {
		return pixff + "-" + string(result)
	}
	return string(result)
}

func getCertificate(driver *driver.KubernetesDriver, namespace, name string) (*Certificate, error) {
	secret, err := driver.Client.CoreV1().Secrets(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	if len(secret.Data[apiv1.TLSCertKey]) == 0 || len(secret.Data[apiv1.TLSPrivateKeyKey]) == 0 {
		return nil, errors.New("invalid secret")
	}
	_, err = tls.X509KeyPair(secret.Data[apiv1.TLSCertKey], secret.Data[apiv1.TLSPrivateKeyKey])
	if err != nil {
		return nil, err
	}
	return &Certificate{
		Cert: string(secret.Data[apiv1.TLSCertKey]),
		Key:  string(secret.Data[apiv1.TLSPrivateKeyKey]),
	}, nil
}

func workerLimit() int {
	n := config.GetInt("WORKER_LIMIT")
	if n > 0 {
		return n
	}
	return 4
}

func ParseCertificateName(n string) (string, string, error) {
	// backward compatibility
	if strings.Contains(n, "_") {
		slice := strings.Split(n, "_")
		if len(slice) != 2 {
			return "", "", errors.New("invalid certificate name")
		}
		return slice[0], slice[1], nil
	}
	if strings.Contains(n, "/") {
		slice := strings.Split(n, "/")
		if len(slice) != 2 {
			return "", "", fmt.Errorf("invalid certificate name, %s", n)
		}
		return slice[0], slice[1], nil
	}
	return "", "", fmt.Errorf("invalid certificate name, %s", n)
}

func getPortInfo(driver *driver.KubernetesDriver) (map[string][]string, error) {
	cm, err := driver.Client.CoreV1().ConfigMaps(config.Get("NAMESPACE")).Get(
		fmt.Sprintf("%s-port-info", config.Get("NAME")), metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	type PortProject []struct {
		Port     string   `json:"port"`
		Projects []string `json:"projects"`
	}
	if cm.Data["range"] != "" {
		var body PortProject
		if err := json.Unmarshal([]byte(cm.Data["range"]), &body); err != nil {
			return nil, err
		}
		var rv = make(map[string][]string)
		for _, i := range body {
			rv[i.Port] = i.Projects
		}
		return rv, nil
	}
	return nil, fmt.Errorf("invalid port-info format, %v", cm.Data)
}

func getPortProject(port int, info map[string][]string) ([]string, error) {
	for k, v := range info {
		if strings.Contains(k, "-") {
			// port range
			parts := strings.Split(k, "-")
			if len(parts) != 2 {
				return nil, fmt.Errorf("invalid port range %s", k)
			}
			s, e := parts[0], parts[1]
			start, err := strconv.Atoi(s)
			if err != nil {
				return nil, err
			}
			end, err := strconv.Atoi(e)
			if err != nil {
				return nil, err
			}
			if start >= end {
				return nil, errors.New("ip range start should less than end")
			}
			if port >= start && port <= end {
				return v, nil
			}
		} else {
			// single port
			single, err := strconv.Atoi(k)
			if err != nil {
				return nil, err
			}
			if single == port {
				return v, nil
			}
		}
	}
	return nil, nil
}

func generatePatchPortProjectPayload(labels map[string]string, desiredProjects []string) []byte {
	newLabels := make(map[string]string)
	//project.cpaas.io/ALL_ALL=true
	for k, v := range labels {
		if !strings.HasPrefix(k, fmt.Sprintf("project.%s", config.Get("DOMAIN"))) {
			newLabels[k] = v
		}
	}
	for _, p := range desiredProjects {
		newLabels[fmt.Sprintf("project.%s/%s", config.Get("DOMAIN"), p)] = "true"
	}
	patchPayloadTemplate :=
		`[{
        "op": "%s",
        "path": "/metadata/labels",
        "value": %s
          }]`

	raw, _ := json.Marshal(newLabels)
	return []byte(fmt.Sprintf(patchPayloadTemplate, "replace", raw))
}

func checkIPV6() bool {
	if config.Get("ENABLE_IPV6") == "true" {
		if _, err := os.Stat("/proc/net/if_inet6"); err != nil {
			if os.IsNotExist(err) {
				return false
			}
		}
		return true
	}
	return false
}
