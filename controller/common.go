package controller

import (
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/golang/glog"

	"alauda_lb/config"
	"alauda_lb/driver"
	"reflect"
)

var (
	SUCCESS              = "success"
	FAILED               = "failed"
	StatusFileParentPath = "/var/run/alb/last_status"
)

func getServiceName(id string, port int) string {
	if port != 0 {
		return fmt.Sprintf("%s-%d", id, port)
	}
	return id
}

func merge(loadBalancers []*LoadBalancer, services []*driver.Service) {
	serviceMap := make(map[string][]*driver.Backend)
	for _, svc := range services {
		name := getServiceName(svc.ServiceID, svc.ContainerPort)
		serviceMap[name] = svc.Backends
	}
	for _, lb := range loadBalancers {
		// lb.Frontends = make(map[]*Frontend)
		for _, ft := range lb.Frontends {
			var rules RuleList
			for _, rule := range ft.Rules {
				if len(rule.Services) == 0 {
					glog.Infof("skip rule %s.", rule.RuleID)
					continue
				}
				rule.BackendGroup = &BackendGroup{
					Name: rule.RuleID,
					Mode: ModeHTTP,
					SessionAffinityPolicy:    rule.SessionAffinityPolicy,
					SessionAffinityAttribute: rule.SessionAffinityAttr,
				}
				totalWeight := 0
				for _, svc := range rule.Services {
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
				for _, svc := range rule.Services {
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
						rule.BackendGroup.Backends = append(rule.BackendGroup.Backends,
							&Backend{
								Address: be.IP,
								Port:    port,
								Weight:  weight,
							})
					}
				}
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

			if ft.ServiceID != "" {
				backends, ok := serviceMap[getServiceName(ft.ServiceID, ft.ContainerPort)]
				if !ok || len(backends) == 0 {
					backends, ok = serviceMap[getServiceName(ft.ServiceID, 0)]
					if !ok || len(backends) == 0 {
						glog.Infof("skip frontend %s because no backend found.",
							ft.String())
						continue
					}
				}
				ft.BackendGroup = &BackendGroup{
					Name: ft.String(),
				}
				if ft.Protocol == ProtocolTCP {
					ft.BackendGroup.Mode = ModeTCP
				} else {
					ft.BackendGroup.Mode = ModeHTTP
				}
				for _, be := range backends {
					port := be.Port
					if port == 0 {
						port = ft.ContainerPort
					}
					ft.BackendGroup.Backends = append(ft.BackendGroup.Backends,
						&Backend{
							Address: be.IP,
							Port:    port,
							Weight:  100,
						})
				}
			}
		}
	}
}

func lbMatch(lb *LoadBalancer, typ, name string) bool {
	if lb.Type == typ {
		switch lb.Type {
		case config.Haproxy, config.Nginx:
			if lb.Name == name {
				return true
			}
		case config.ELB, config.SLB, config.CLB:
			return true
		}
	}
	return false
}

func filterLoadbalancers(loadBalancers []*LoadBalancer, loadBalancerType, name string) []*LoadBalancer {
	res := make([]*LoadBalancer, 0, len(loadBalancers))
	for _, l := range loadBalancers {
		if lbMatch(l, loadBalancerType, name) {
			res = append(res, l)
		}
	}
	return res
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

func generateConfig(loadbalancer *LoadBalancer) Config {
	cfgLocker.Lock()
	defer cfgLocker.Unlock()
	result := Config{
		Name:           loadbalancer.Name,
		Address:        loadbalancer.Address,
		BindAddress:    loadbalancer.BindAddress,
		LoadBalancerID: loadbalancer.LoadBalancerID,
		Frontends:      make(map[int]*Frontend),
		BackendGroup:   []*BackendGroup{},
	}

	for _, ft := range loadbalancer.Frontends {
		isValid := false
		if ft.Protocol == ProtocolHTTP || ft.Protocol == ProtocolHTTPS {
			for _, rule := range ft.Rules {
				rule.Domain = strings.ToLower(rule.Domain)
				rule.URL = strings.ToLower(rule.URL)
				rule.Regexp = generateRegexp(ft, rule)
				result.BackendGroup = append(result.BackendGroup, rule.BackendGroup)
				isValid = true
			}
		}
		if ft.BackendGroup != nil {
			result.BackendGroup = append(result.BackendGroup, ft.BackendGroup)
			isValid = true
		}

		if ft.Protocol == ProtocolHTTPS {
			ft.CertificateFiles = make(map[string]string)
			if ft.CertificateID != "" {
				if config.Get("NAME") != "alb-xlb" {
					path, err := downloadCertificate(ft.CertificateID)
					if err == nil {
						ft.CertificateFiles[ft.CertificateID] = path
					}
				} else {
					ft.CertificateFiles[ft.CertificateID] = ft.CertificateName
				}

			}
			for _, rule := range ft.Rules {
				if rule.CertificateID != "" {
					if config.Get("NAME") != "alb-xlb" {
						path, err := downloadCertificate(rule.CertificateID)
						if err == nil {
							ft.CertificateFiles[rule.CertificateID] = path
						} else {
							glog.Errorf(err.Error())
						}
					} else {
						ft.CertificateFiles[ft.CertificateID] = ft.CertificateName
					}
				}
			}
			if len(ft.CertificateFiles) == 0 {
				isValid = false
			}
		}

		if isValid {
			result.Frontends[ft.Port] = ft
		} else {
			glog.Infof("Skip invalid frontend %s.", ft.String())
		}
	} // end of  _, ft := range loadbalancer.Frontends

	if config.Get("RECORD_POST_BODY") == "true" {
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
		return SUCCESS
	}
	return FAILED
}

func isDownloaded(filename string) bool {
	_, err := os.Stat(filename)
	if err == nil || os.IsExist(err) {
		return true
	}
	return false
}

type Project struct {
	UUID string `json:"uuid"`
	Name string `json:"name"`
}

func getProjectNames() ([]string, error) {
	url := fmt.Sprintf("%s/v1/projects/%s", config.Get("JAKIRO_ENDPOINT"), config.Get("NAMESPACE"))
	resp, body, errs := JakiroRequest.Get(url).Set("Authorization", fmt.Sprintf("Token %s", config.Get("TOKEN"))).End()
	if len(errs) != 0 {
		return nil, errs[0]
	}
	if resp.StatusCode == 403 {
		// project not enabled add empty project name
		return []string{""}, nil
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("request project names failed %d with %s", resp.StatusCode, body)
	}
	var projects []*Project
	if err := json.Unmarshal([]byte(body), &projects); err != nil {
		return nil, err
	}
	projectNames := []string{}
	for _, project := range projects {
		projectNames = append(projectNames, project.Name)
	}
	// add default empty project name
	projectNames = append(projectNames, "")
	glog.Infof("Get projects %v", projectNames)
	return projectNames, nil
}

func downloadCertificate(certificateID string) (string, error) {
	certificatePath := filepath.Join(config.Get("CERTIFICATE_DIRECTORY"), certificateID)
	certificatePathTemp := certificatePath + ".temp"

	if !isDownloaded(certificatePath) {
		glog.Info("Certificate file does not exist, start loading...")
		url := fmt.Sprintf(
			"%s/v1/certificates/%s/%s?project_name=default",
			config.Get("JAKIRO_ENDPOINT"),
			config.Get("NAMESPACE"),
			certificateID,
		)
		certificate := Certificate{}
		projectNames, err := getProjectNames()
		if err != nil {
			glog.Error(err)
			return "", err
		}
		for _, name := range projectNames {
			resp, body, errs := JakiroRequest.Get(url).
				Query("with_content=true").
				Query(fmt.Sprintf("project_name=%s", name)).
				Set("Authorization", fmt.Sprintf("Token %s", config.Get("TOKEN"))).
				End()
			if len(errs) > 0 {
				glog.Error(errs[0].Error())
				return "", errs[0]
			}

			if resp.StatusCode == 404 {
				continue
			}

			if resp.StatusCode != 200 {
				glog.Error(body)
				return "", errors.New(body)
			}
			if err := json.Unmarshal([]byte(body), &certificate); err != nil {
				glog.Error(err.Error())
				return "", err
			} else {
				break
			}
		}

		if certificate.Name == "" {
			return "", fmt.Errorf("certificate %s not found", certificateID)
		}

		if certificate.Status != "Normal" {
			return "", fmt.Errorf("certificate %s is invalid", certificateID)
		}
		f, err := os.Create(certificatePathTemp)
		if err != nil {
			glog.Error(err.Error())
			return "", err
		}
		defer f.Close()
		if _, err = f.WriteString(certificate.PublicCert + "\n"); err != nil {
			glog.Error(err.Error())
			return "", err
		}
		if _, err = f.WriteString(certificate.PrivateKey); err != nil {
			glog.Error(err.Error())
			return "", err
		}
		f.Sync()
		if err = os.Rename(certificatePathTemp, certificatePath); err != nil {
			glog.Errorf("failed to replace certificate %s. | %s", certificateID, err.Error())
			return "", err
		}
	}

	return certificatePath, nil
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
