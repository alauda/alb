package controller

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"github.com/golang/glog"

	"alb2/config"
	"alb2/driver"
)

const (
	ProtocolHTTP  = "http"
	ProtocolHTTPS = "https"
	ProtocolTCP   = "tcp"
	ProtocolUDP   = "udp"

	PolicySIPHash = "sip-hash"
	PolicyCookie  = "cookie"
)

var LastConfig = ""
var LastFailure = false

type Controller interface {
	GetLoadBalancerType() string
	GenerateConf() error
	ReloadLoadBalancer() error
	GC() error
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

type Certificate struct {
	Cert string `json:"cert"`
	Key  string `json:"key"`
}

type Frontend struct {
	RawName        string            `json:"-"`
	LoadBalancerID string            `json:"load_balancer_id"`
	Port           int               `json:"port"`
	Protocol       string            `json:"protocol"`
	Rules          RuleList          `json:"rules"`
	Services       []*BackendService `json:"services"`

	BackendGroup    *BackendGroup `json:"-"`
	CertificateName string        `json:"certificate_name"`
}

func (ft *Frontend) String() string {
	return fmt.Sprintf("%s-%d-%s", ft.LoadBalancerID, ft.Port, ft.Protocol)
}

type Backend struct {
	Address string
	Port    int
	Weight  int
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

type BackendService struct {
	ServiceID     string `json:"service_id"`
	ContainerPort int    `json:"container_port"`
	Weight        int    `json:"weight"`
}

type Rule struct {
	RuleID   string `json:"rule_id"`
	Priority int64  `json:"priority"`
	Type     string `json:"type"`
	Domain   string `json:"domain"`
	URL      string `json:"url"`
	DSL      string `json:"dsl"`
	// CertificateName = namespace_secretname
	CertificateName       string            `json:"certificate_name"`
	RewriteTarget         string            `json:"rewrite_target"`
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
	CertificateMap map[string]Certificate
}

var (
	//ErrStandAlone will be return if do something that not allowed in stand mode
	ErrStandAlone = errors.New("operation is not allowed in stand alone mode")
)

//IsNewK8sCluster return true if cluser is new kubernetes cluseter
func IsNewK8sCluster() (bool, error) {
	return true, nil
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

	d, err := driver.GetDriver()
	if err != nil {
		return nil, err
	}
	alb, err := d.LoadALBbyName(config.Get("NAMESPACE"), config.Get("NAME"))
	if err != nil {
		glog.Error(err)
		return []*LoadBalancer{}, nil
	}

	lb, err := MergeNew(alb)
	if err != nil {
		glog.Error(err)
		return nil, err
	}
	var loadBalancers = []*LoadBalancer{
		lb,
	}

	interval, err := strconv.Atoi(config.Get("INTERVAL"))
	if err != nil {
		glog.Error(err)
		interval = 5
	}
	nextFetchTime = time.Now().Add(time.Duration(interval) * time.Second)
	loadBalancersCache, _ = json.Marshal(loadBalancers)
	glog.V(3).Infof("Get Loadbalancers: %s", string(loadBalancersCache))
	return loadBalancers, err
}

func GetController() (Controller, error) {
	d, err := driver.GetDriver()
	if err != nil {
		return nil, err
	}

	switch config.Get("LB_TYPE") {
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
	default:
		return nil, fmt.Errorf("Unsupport lb type %s only support nginx. Will support elb, slb, clb in the future", config.Get("LB_TYPE"))
	}
}
