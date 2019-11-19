package controller

import (
	"crypto/md5"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

	"github.com/golang/glog"
	"github.com/thoas/go-funk"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"alb2/config"
	"alb2/driver"
	"alb2/utils"
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
					glog.Warningf("skip rule %s.", rule.RuleID)
					continue
				}
				rule.BackendGroup = &BackendGroup{
					Name: rule.RuleID,
					// bg.mode dont care whether http or https
					Mode:                     ModeHTTP,
					SessionAffinityPolicy:    rule.SessionAffinityPolicy,
					SessionAffinityAttribute: rule.SessionAffinityAttr,
				}
				rule.BackendGroup.Backends = generateBackend(serviceMap, rule.Services)
				if len(rule.BackendGroup.Backends) > 0 {
					rules = append(rules, rule)
				}
			}
			if len(rules) > 0 {
				sort.Sort(rules)
				ft.Rules = rules
			} else {
				ft.Rules = RuleList{}
			}

			if len(ft.Services) == 0 {
				glog.Warningf("skip frontend %s because no default service found.",
					ft.String())
				continue
			}
			ft.BackendGroup.Backends = generateBackend(serviceMap, ft.Services)

			if ft.Protocol == ProtocolTCP {
				ft.BackendGroup.Mode = ModeTCP
			} else {
				ft.BackendGroup.Mode = ModeHTTP
			}
		}
	}
}

func generateRegexp(ft *Frontend, rule *Rule) string {
	domain := "[^/]+"
	if rule.Domain != "" {
		domain = rule.Domain
	}
	url := ".*"
	if rule.URL != "" {
		if strings.HasPrefix(rule.URL, "^") {
			url = rule.URL[1:]
		} else {
			url = rule.URL + ".*"
		}
	}
	var reg string
	if (ft.Protocol == ProtocolHTTP && ft.Port == 80) ||
		(ft.Protocol == ProtocolHTTPS && ft.Port == 443) {
		reg = fmt.Sprintf("^%s(:%d)?%s$", domain, ft.Port, url)
	} else {
		reg = fmt.Sprintf("^%s:%d%s$", domain, ft.Port, url)
	}
	return reg
}

var cfgLocker sync.Mutex

func generateConfig(loadbalancer *LoadBalancer, driver *driver.KubernetesDriver) Config {
	cfgLocker.Lock()
	defer cfgLocker.Unlock()
	result := Config{
		Name:           loadbalancer.Name,
		Address:        loadbalancer.Address,
		BindAddress:    loadbalancer.BindAddress,
		LoadBalancerID: loadbalancer.LoadBalancerID,
		Frontends:      make(map[int]*Frontend),
		BackendGroup:   []*BackendGroup{},
		CertificateMap: make(map[string]Certificate),
		TweakHash:      loadbalancer.TweakHash,
	}
	listenTCPPorts, err := utils.GetListenTCPPorts()
	if err != nil {
		glog.Error(err)
	}
	for _, ft := range loadbalancer.Frontends {
		conflict := false
		for _, port := range listenTCPPorts {
			if ft.Port == port {
				glog.Warningf("skip port: %d due to conflict", ft.Port)
				conflict = true
				glog.Errorf("skip port: %d has conflict", ft.Port)
				break
			}
		}
		if err := driver.UpdateFrontendStatus(ft.RawName, conflict); err != nil {
			glog.Error(err)
		}
		if conflict {
			// skip conflict port
			continue
		}
		glog.Infof("generate config for ft %d %s, have %d rules", ft.Port, ft.Protocol, len(ft.Rules))
		isValid := false
		isHTTP := ft.Protocol == ProtocolHTTP
		isHTTPS := ft.Protocol == ProtocolHTTPS
		if isHTTP || isHTTPS {
			if isHTTPS && ft.CertificateName != "" {
				slice := strings.Split(ft.CertificateName, "_")
				secretNs := slice[0]
				secretName := slice[1]
				cert, err := getCertificate(driver, secretNs, secretName)
				if err != nil {
					glog.Warningf("get cert failed, %+v", err)
					continue
				}
				// default cert for port ft.Port
				result.CertificateMap[strconv.Itoa(ft.Port)] = *cert
			}
			for _, rule := range ft.Rules {
				if isHTTPS && rule.Domain != "" && rule.CertificateName != "" {
					slice := strings.Split(rule.CertificateName, "_")
					secretNs := slice[0]
					secretName := slice[1]
					cert, err := getCertificate(driver, secretNs, secretName)
					if err != nil {
						glog.Warningf("get cert failed, %+v", err)
						continue
					}
					if existCert, ok := result.CertificateMap[strings.ToLower(rule.Domain)]; ok {
						if existCert.Cert != cert.Cert || existCert.Key != cert.Key {
							glog.Warningf("declare different cert for host %s", strings.ToLower(rule.Domain))
							continue
						}
					}
					result.CertificateMap[strings.ToLower(rule.Domain)] = *cert
				}
				rule.Domain = strings.ToLower(rule.Domain)
				rule.Regexp = generateRegexp(ft, rule)
				result.BackendGroup = append(result.BackendGroup, rule.BackendGroup)
				isValid = true
			}
		}
		if ft.BackendGroup != nil && len(ft.BackendGroup.Backends) > 0 {
			// FIX: http://jira.alaudatech.com/browse/DEV-16954
			// remove duplicate upstream
			if !funk.Contains(result.BackendGroup, ft.BackendGroup) {
				result.BackendGroup = append(result.BackendGroup, ft.BackendGroup)
			}
			isValid = true
		}

		if isValid {
			result.Frontends[ft.Port] = ft
		} else {
			glog.Warningf("Skip invalid frontend %s.", ft.String())
		}
	} // end of  _, ft := range loadbalancer.Frontends

	if config.Get("RECORD_POST_BODY") == "true" {
		// nginx not support record request body alb2.x
		result.RecordPostBody = true
	}
	return result
}

func sameFiles(file1, file2 string) bool {
	sum1, err := fileMd5(file1)
	if err != nil {
		glog.Error(err.Error())
		return false
	}
	sum2, err := fileMd5(file2)
	if err != nil {
		glog.Error(err.Error())
		return false
	}

	return sum1 == sum2
}

func fileMd5(file string) (string, error) {
	f, err := os.Open(file)
	if err != nil {
		glog.Error(err.Error())
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
			glog.Errorf("create new status file failed %s", err.Error())
			return err
		}
		f.Close()
	}

	reversStatusFilePath := path.Join(statusFileParentPath, reverseStatus(status))
	if _, err := os.Stat(reversStatusFilePath); err == nil {
		err := os.Remove(reversStatusFilePath)
		if err != nil {
			glog.Errorf("remove old status file failed %s", err.Error())
			return err
		}
	}
	return nil
}

func getLastReloadStatus(statusFileParentPath string) string {
	successStatusFilePath := path.Join(statusFileParentPath, SUCCESS)
	if _, err := os.Stat(successStatusFilePath); err == nil {
		glog.Info("last reload status", SUCCESS)
		return SUCCESS
	}
	glog.Info("last reload status", FAILED)
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
