package controller

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"
	"github.com/parnurzeal/gorequest"

	"alauda_lb/config"
	"alauda_lb/driver"
)

const (
	ProtocolHTTP  = "http"
	ProtocolHTTPS = "https"
	ProtocolTCP   = "tcp"
	ProtocolUDP   = "udp"

	PolicySIPHash = "sip-hash"
	PolicyCookie  = "cookie"
)

var (
	JakiroRequest = gorequest.New().Timeout(30 * time.Second)
)

var LastConfig = ""
var LastFailure = false
var lastCheckTime time.Time

type Controller interface {
	GetLoadBalancerType() string
	GenerateConf() error
	ReloadLoadBalancer() error
}

func CheckProcessAlive(process string) (string, error) {
	out, err := exec.Command("/usr/bin/pgrep", "-f", process).CombinedOutput()
	return string(out), err
}

type Domain struct {
	Domain   string `json:"domain"`
	Type     string `json:"type,omitempty"`
	Disabled bool   `json:"disabled"`
}

type LoadBalancer struct {
	Name           string      `json:"name"`
	Address        string      `json:"address"`
	BindAddress    string      `json:"bind_address"`
	LoadBalancerID string      `json:"iaas_id"`
	Type           string      `json:"type"`
	Version        int         `json:"version"`
	Frontends      []*Frontend `json:"frontends"`
	DomainInfo     []Domain    `json:"domain_info"`
}

func (lb *LoadBalancer) String() string {
	r, err := json.Marshal(lb)
	if err != nil {
		glog.Errorf("Error to parse lb: %s", err.Error())
		return ""
	}
	return string(r)
}

type CertificateInfo struct {
	CertificateID   string
	CertificateName string
	CertificatePath string
}

type Frontend struct {
	LoadBalancerID  string   `json:"load_balancer_id"`
	Port            int      `json:"port"`
	Protocol        string   `json:"protocol"`
	Rules           RuleList `json:"rules"`
	ServiceID       string   `json:"service_id"`
	ContainerPort   int      `json:"container_port"`
	CertificateID   string   `json:"certificate_id"`
	CertificateName string   `json:"certificate_name"`

	BackendGroup     *BackendGroup     `json:"-"`
	CertificateFiles map[string]string `json:"-"`
	ready            bool
}

func (ft *Frontend) String() string {
	return fmt.Sprintf("%s-%d-%s", ft.LoadBalancerID, ft.Port, ft.Protocol)
}

type Backend struct {
	Address string
	Port    int
	Weight  int
}

func (be *Backend) Name() string {
	var addrStr string
	if ip := net.ParseIP(be.Address); ip != nil {
		var ipnum uint32
		if len(ip) == 16 { //ipv6
			ipnum = binary.BigEndian.Uint32(ip[12:16])
		} else {
			ipnum = binary.BigEndian.Uint32(ip)
		}
		addrStr = strconv.Itoa(int(ipnum))
	} else {
		addrStr = be.Address
	}
	return fmt.Sprintf("%s_%d", addrStr, be.Port)
}

const (
	ModeTCP  = "tcp"
	ModeHTTP = "http"
)

type BackendGroup struct {
	Name                     string
	SessionAffinityPolicy    string
	SessionAffinityAttribute string
	Mode                     string
	Backends                 []*Backend
}

type ByBackendGroup []*BackendGroup

func (b ByBackendGroup) Len() int {
	return len(b)
}

func (b ByBackendGroup) Swap(i, j int) {
	b[i], b[j] = b[j], b[i]
}

func (b ByBackendGroup) Less(i, j int) bool {
	return b[i].Name < b[j].Name
}

type BackendService struct {
	ServiceID     string `json:"service_id"`
	ContainerPort int    `json:"container_port"`
	Weight        int    `json:"weight"`
}

type Rule struct {
	RuleID                string            `json:"rule_id"`
	Priority              int64             `json:"priority"`
	Type                  string            `json:"type"`
	Domain                string            `json:"domain"`
	URL                   string            `json:"url"`
	DSL                   string            `json:"dsl"`
	CertificateID         string            `json:"certificate_id"`
	CertificateName       string            `json:"certificate_name"`
	Description           string            `json:"description"`
	SessionAffinityPolicy string            `json:"session_affinity_policy"`
	SessionAffinityAttr   string            `json:"session_affinity_attribute"`
	Services              []*BackendService `json:"services"`

	BackendGroup *BackendGroup `json:"-"`
	Regexp       string        `json:"-"`
}

type RuleList []*Rule

func (rl RuleList) Len() int {
	return len(rl)
}

func (rl RuleList) Swap(i, j int) {
	rl[i], rl[j] = rl[j], rl[i]
}

func (rl RuleList) Less(i, j int) bool {
	return rl[i].Priority > rl[j].Priority
}

type Config struct {
	Name           string
	Address        string
	BindAddress    string
	LoadBalancerID string
	Frontends      map[int]*Frontend
	BackendGroup   []*BackendGroup
	RecordPostBody bool
}

type Certificate struct {
	UUID         string    `json:"uuid"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	PrivateKey   string    `json:"private_key"`
	PublicCert   string    `json:"public_cert"`
	IsUsed       bool      `json:"is_used"`
	Status       string    `json:"status"`
	ServiceCount int       `json:"service_count"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type RegionInfo struct {
	Name             string `json:"name"`
	ID               string `json:"id"`
	ContainerManager string `json:"container_manager"`
	PlatformVersion  string `json:"platform_version"`
}

var regionInfo *RegionInfo

var (
	//ErrStandAlone will be return if do something that not allowed in stand mode
	ErrStandAlone = errors.New("operation is not allowed in stand alone mode")
)

func LoadRegionInfo() (*RegionInfo, error) {
	if config.IsStandalone() {
		glog.Error("can't load region information when run in stand alone mode")
		return nil, ErrStandAlone
	}
	if regionInfo != nil {
		return regionInfo, nil
	}
	defer func(start time.Time) {
		dur := time.Now().Sub(start)
		glog.Infof("Fetch region info used %.3f seconds.", float64(dur)/float64(time.Second))
	}(time.Now())
	url := fmt.Sprintf("%s/v1/regions/%s/%s",
		config.Get("JAKIRO_ENDPOINT"),
		config.Get("NAMESPACE"),
		config.Get("REGION_NAME"))
	resp, body, errs := gorequest.New().Get(url).
		Set("Authorization", fmt.Sprintf("Token %s", config.Get("TOKEN"))).
		Timeout(time.Second * time.Duration(config.GetInt("timeout"))).
		End()
	if len(errs) > 0 {
		glog.Error(errs[0])
		return nil, errs[0]
	}
	if resp.StatusCode != 200 {
		glog.Error(body)
		return nil, errors.New(body)
	}
	err := json.Unmarshal([]byte(body), &regionInfo)
	if err != nil {
		glog.Error(err)
		return nil, err
	}
	glog.Info("Get region info: ", body)
	return regionInfo, nil
}

//IsNewK8sCluster return true if cluser is new kubernetes cluseter
func IsNewK8sCluster() (bool, error) {
	if config.IsStandalone() {
		return true, nil
	}

	if config.GetBool("k8s_v3") {
		//Set env ALB_K8S_v3=true to enable it
		return true, nil
	}
	info, err := LoadRegionInfo()
	if err == nil {
		return (strings.EqualFold(info.ContainerManager, "kubernetes") &&
			strings.EqualFold(info.PlatformVersion, "v3")), nil
	}
	return false, err
}

//FetchLBFromMirana2 get load balancers informations from mirana2 in global.
func FetchLBFromMirana2() ([]*LoadBalancer, error) {
	if config.IsStandalone() {
		return nil, ErrStandAlone
	}
	var err error
	defer printFuncLog("FetchLBFromMirana2", time.Now(), err)
	url := fmt.Sprintf("%s/v1/load_balancers/%s", config.Get("JAKIRO_ENDPOINT"), config.Get("NAMESPACE"))
	resp, body, errs := JakiroRequest.Get(url).
		Query(fmt.Sprintf("region_name=%s&frontend=true", config.Get("REGION_NAME"))).
		Set("Authorization", fmt.Sprintf("Token %s", config.Get("TOKEN"))).
		Timeout(time.Second * time.Duration(config.GetInt("timeout"))).
		End()
	if len(errs) > 0 {
		glog.Error(errs[0].Error())
		return nil, errs[0]
	}
	if resp.StatusCode != 200 {
		glog.Errorf("Request to %s get %d: %s", url, resp.StatusCode, body)
		return nil, errors.New(body)
	}
	glog.Infof("Request to jakiro %s", resp.Request.URL)
	glog.Infof("Jakiro Response body is %d bytes", len(body))
	var loadBalancers []*LoadBalancer
	err = json.Unmarshal([]byte(body), &loadBalancers)
	return loadBalancers, err
}

var loadBalancersCache []byte
var nextFetchTime time.Time
var infoLock sync.Mutex

//FetchLoadBalancersInfo return loadbalancer info from cache, mirana2 or apiserver
func FetchLoadBalancersInfo() ([]*LoadBalancer, error) {
	infoLock.Lock()
	defer infoLock.Unlock()
	if time.Now().Before(nextFetchTime) && loadBalancersCache != nil {
		var lbs []*LoadBalancer
		//make sure always return a copy of loadbalaners
		err := json.Unmarshal(loadBalancersCache, &lbs)
		if err != nil {
			// should never happen
			glog.Error(err)
			panic(err)
		}
		return lbs, nil
	}

	newK8s, err := IsNewK8sCluster()
	if err != nil {
		return nil, err
	}

	var loadBalancers = []*LoadBalancer{}
	if newK8s {
		albs, err := FetchALBInfo()
		if err != nil {
			return nil, err
		}
		for _, alb := range albs {
			loadBalancers = append(loadBalancers, alb.Spec)
		}
	} else {
		loadBalancers, err = FetchLBFromMirana2()
		if err != nil {
			return nil, err
		}
	}

	interval, err := strconv.Atoi(config.Get("INTERVAL"))
	if err != nil {
		glog.Error(err)
		interval = 5
	}
	nextFetchTime = time.Now().Add(time.Duration(interval) * time.Second)
	loadBalancersCache, _ = json.Marshal(loadBalancers)
	return loadBalancers, err
}

func GetController() (Controller, error) {
	d, err := driver.GetDriver()
	if err != nil {
		return nil, err
	}

	switch config.Get("LB_TYPE") {
	case config.Haproxy:
		return &HaproxyController{
			TemplatePath:  config.Get("HAPROXY_TEMPLATE_PATH"),
			NewConfigPath: config.Get("NEW_CONFIG_PATH"),
			OldConfigPath: config.Get("OLD_CONFIG_PATH"),
			BackendType:   d.GetType(),
			BinaryPath:    config.Get("HAPROXY_BIN_PATH"),
			Driver:        d}, nil
	case config.Nginx:
		return &NginxController{
			TemplatePath:  config.Get("NGINX_TEMPLATE_PATH"),
			NewConfigPath: config.Get("NEW_CONFIG_PATH"),
			OldConfigPath: config.Get("OLD_CONFIG_PATH"),
			NewPolicyPath: config.Get("NEW_POLICY_PATH"),
			OldPolicyPath: config.Get("OLD_POLICY_PATH"),
			BackendType:   d.GetType(),
			BinaryPath:    config.Get("NGINX_BIN_PATH"),
			Driver:        d}, nil
	case config.ELB:
		elbController := &ElbController{Driver: d}
		elbController.init()
		return elbController, nil
	case config.SLB:
		slbController := &SlbController{Driver: d}
		slbController.init()
		return slbController, nil
	case config.CLB:
		clbController := &ClbController{Driver: d}
		clbController.init()
		return clbController, nil
	default:
		return nil, fmt.Errorf("Unsupport lb type %s only support elb, slb,clb and haproxy", config.Get("LB_TYPE"))
	}
}

// SyncLoadBalancersAndScheduler create NodePort when
// a deployment was bind with a load balancer,
// while it will be deleted if the deployment was unbind.
func SyncLoadBalancersAndScheduler() error {
	drv, err := driver.GetDriver()
	if err != nil {
		glog.Errorf("Failed to get driver: %s", err)
		return err
	}
	if drv.GetType() != config.Kubernetes {
		return nil
	}

	services, err := drv.ListService()
	if err != nil {
		glog.Errorf("Failed to get service list: %s", err)
		return err
	}
	loadbalancers, err := FetchLoadBalancersInfo()
	if err != nil {
		glog.Errorf("Failed to get load balancers: %s", err)
		return err
	}
	loadbalancers = filterLoadbalancers(loadbalancers,
		config.Get("LB_TYPE"), config.Get("NAME"))

	toDelete := make(map[string]*driver.Service)
	for _, svc := range services {
		name := svc.ServiceName
		if config.GetBool("USE_ENDPOINT") {
			name = fmt.Sprintf("alauda-%s-0", svc.ServiceID)
		}
		toDelete[name] = svc
	}
	newServices := make(map[string]*BackendService, 0)
	done := make(map[string]bool, 0)
	for _, lb := range loadbalancers {
		for _, ft := range lb.Frontends {
			for _, rule := range ft.Rules {
				for _, svc := range rule.Services {
					port := svc.ContainerPort
					if config.GetBool("USE_ENDPOINT") {
						port = 0
					}
					name := fmt.Sprintf("alauda-%s-%d", svc.ServiceID, port)
					if _, ok := done[name]; ok {
						continue
					}
					if _, ok := toDelete[name]; !ok {
						newServices[name] = svc
					} else {
						// existed services, remove from the deletion list
						delete(toDelete, name)
					}
					done[name] = true
				}
			}
			if ft.ServiceID != "" {
				svc := &BackendService{
					ServiceID:     ft.ServiceID,
					ContainerPort: ft.ContainerPort,
					Weight:        100,
				}
				port := svc.ContainerPort
				if config.GetBool("USE_ENDPOINT") {
					port = 0
				}
				name := fmt.Sprintf("alauda-%s-%d", svc.ServiceID, port)
				if _, ok := done[name]; ok {
					continue
				}
				if _, ok := toDelete[name]; !ok {
					newServices[name] = svc
				} else {
					delete(toDelete, name)
				}
				done[name] = true
			}
		}
	}
	for _, svc := range toDelete {
		if svc.Owner == config.Get("NAME") {
			glog.Infof("delete nodeport %s created by %s", svc.ServiceName, svc.Owner)
			// only delete nodeports created by itself or by old version alb
			if err := drv.DeleteNodePort(svc.ServiceName, svc.Namespace); err != nil {
				glog.Error(err)
			}
		}
	}
	labelServiceID := config.Get("LABEL_SERVICE_ID")
	labelOwner := config.Get("LABEL_CREATOR")
	for name, svc := range newServices {
		nodePort := &driver.NodePort{
			Name: name,
			Selector: map[string]string{
				labelServiceID: svc.ServiceID,
			},
			Labels: map[string]string{
				labelServiceID: svc.ServiceID,
				labelOwner:     config.Get("NAME"),
			},
			Ports: []int{svc.ContainerPort},
		}
		err := drv.CreateNodePort(nodePort)
		if err != nil {
			//It may failed if it has been created by another alb
			glog.Error(err)
		}
	}

	return nil
}
