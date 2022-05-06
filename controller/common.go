package controller

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"os"
	"path"
	"reflect"
	"strconv"
	"strings"
	"time"

	"alauda.io/alb2/config"
	. "alauda.io/alb2/controller/types"
	"alauda.io/alb2/driver"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

var (
	SUCCESS = "success"
	FAILED  = "failed"
)

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}

func generateServiceKey(ns string, name string, protocol apiv1.Protocol, svcPort int) string {
	key := fmt.Sprintf("%s-%s-%s-%d", ns, name, protocol, svcPort)
	return strings.ToLower(key)
}

// 找到service 对应的后端
func generateBackend(backendMap map[string][]*driver.Backend, services []*BackendService, protocol apiv1.Protocol) Backends {
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
		name := generateServiceKey(svc.ServiceNs, svc.ServiceName, protocol, svc.ServicePort)
		backends, ok := backendMap[name]
		// some rule such as redirect ingress will use a fake service.
		if !ok || len(backends) == 0 {
			continue
		}
		//100 is the max weigh in SLB
		weight := int(math.Floor(float64(svc.Weight*100)/float64(totalWeight*len(backends)) + 0.5))
		if weight == 0 && svc.Weight != 0 {
			weight = 1
		}
		for _, be := range backends {
			port := be.Port
			if port == 0 {
				klog.Warningf("invalid backend port 0 svc: %+v", svc)
				continue
			}
			bes = append(bes,
				&Backend{
					Address:     be.IP,
					Port:        port,
					Weight:      weight,
					Protocol:    be.Protocol,
					AppProtocol: be.AppProtocol,
				})
		}
	}
	return Backends(bes)
}

func sameFiles(file1, file2 string) bool {
	sum1, err := fileSha256(file1)
	if err != nil {
		klog.Warning(err.Error())
		return false
	}
	sum2, err := fileSha256(file2)
	if err != nil {
		klog.Warning(err.Error())
		return false
	}

	return sum1 == sum2
}

func fileSha256(file string) (string, error) {
	f, err := os.Open(file)
	if err != nil {
		return "", err
	}
	sha256h := sha256.New()
	io.Copy(sha256h, f)
	return fmt.Sprintf("%x", sha256h.Sum(nil)), nil
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

func getCertificate(driver *driver.KubernetesDriver, namespace, name string) (*Certificate, *CaCertificate, error) {
	secret, err := driver.Client.CoreV1().Secrets(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return nil, nil, err
	}
	if len(secret.Data[apiv1.TLSCertKey]) == 0 || len(secret.Data[apiv1.TLSPrivateKeyKey]) == 0 {
		return nil, nil, errors.New("invalid secret")
	}
	_, err = tls.X509KeyPair(secret.Data[apiv1.TLSCertKey], secret.Data[apiv1.TLSPrivateKeyKey])
	if err != nil {
		return nil, nil, err
	}
	if len(secret.Data[CaCert]) == 0 {
		return &Certificate{
			Cert: string(secret.Data[apiv1.TLSCertKey]),
			Key:  string(secret.Data[apiv1.TLSPrivateKeyKey]),
		}, nil, nil
	}
	return &Certificate{
		Cert: string(secret.Data[apiv1.TLSCertKey]),
		Key:  string(secret.Data[apiv1.TLSPrivateKeyKey]),
	}, &CaCertificate{Cert: string(secret.Data[CaCert])}, nil
}

func mergeFullchainCert(cert *Certificate, caCert *CaCertificate) *Certificate {
	var fullChainCert string
	if strings.HasSuffix(cert.Cert, "\n") {
		fullChainCert = strings.Join([]string{cert.Cert, caCert.Cert}, "")
	} else {
		fullChainCert = strings.Join([]string{cert.Cert, caCert.Cert}, "\n")
	}
	return &Certificate{
		Cert: fullChainCert,
		Key:  cert.Key,
	}
}

func workerLimit() int {
	n := config.GetInt("WORKER_LIMIT")
	if n > 0 {
		return n
	}
	return 4
}

func cpu_preset() int {
	return config.GetInt("CPU_LIMIT")
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
		context.TODO(),
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
