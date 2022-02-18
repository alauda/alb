package controller

import (
	"alauda.io/alb2/config"
	"alauda.io/alb2/driver"
	v1 "alauda.io/alb2/pkg/apis/alauda/v1"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"k8s.io/klog/v2"
)

const (
	SubsystemHTTP   = "http"
	SubsystemStream = "stream"

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

func GetProcessId() (string, error) {
	process := "/nginx/nginx-pid/nginx.pid"
	out, err := ioutil.ReadFile(process)
	if err != nil {
		klog.Errorf("nginx process is not started: %s", err.Error())
		return "", err
	}
	return string(out), err
}

type Domain struct {
	Domain   string `json:"domain"`
	Type     string `json:"type,omitempty"`
	Disabled bool   `json:"disabled"`
}

type LoadBalancer struct {
	Labels     map[string]string `json:"-"`
	Name       string            `json:"name"`
	Address    string            `json:"address"`
	Type       string            `json:"type"`
	Version    int               `json:"version"`
	Frontends  []*Frontend       `json:"frontends"`
	DomainInfo []Domain          `json:"domain_info"`
	TweakHash  string            `json:"-"`
}

func (lb *LoadBalancer) String() string {
	r, err := json.Marshal(lb)
	if err != nil {
		klog.Errorf("Error to parse lb: %s", err.Error())
		return ""
	}
	return string(r)
}

type Certificate struct {
	Cert string `json:"cert"`
	Key  string `json:"key"`
}

type Frontend struct {
	Labels          map[string]string `json:"-"`
	RawName         string            `json:"-"`        // ft name
	AlbName         string            `json:"alb_name"` // alb name
	Port            int               `json:"port"`
	Protocol        v1.FtProtocol     `json:"protocol"` // ft的协议 http/https/tcp/udp tcp和udp代表stream mode
	Rules           RuleList          `json:"rules"`
	Services        []*BackendService `json:"services"`         // ft的默认后端路由
	BackendProtocol string            `json:"backend_protocol"` // 这个默认后端路由的协议
	BackendGroup    *BackendGroup     `json:"-"`                // 默认后端路由的pod地址
	CertificateName string            `json:"certificate_name"`
	Conflict        bool              `json:"-"`
}

func (ft *Frontend) String() string {
	return fmt.Sprintf("%s-%d-%s", ft.AlbName, ft.Port, ft.Protocol)
}

func (ft *Frontend) IsTcpBaseProtocol() bool {
	return ft.Protocol == v1.FtProtocolHTTP ||
		ft.Protocol == v1.FtProtocolHTTPS ||
		ft.Protocol == v1.FtProtocolTCP
}

func (ft *Frontend) IsStreamMode() bool {
	return ft.Protocol == v1.FtProtocolTCP || ft.Protocol == v1.FtProtocolUDP
}

func (ft *Frontend) IsHttpMode() bool {
	return ft.Protocol == v1.FtProtocolHTTP || ft.Protocol == v1.FtProtocolHTTPS
}

func (ft *Frontend) IsValidProtocol() bool {
	return ft.Protocol == v1.FtProtocolHTTP ||
		ft.Protocol == v1.FtProtocolHTTPS ||
		ft.Protocol == v1.FtProtocolTCP ||
		ft.Protocol == v1.FtProtocolUDP
}

type Backend struct {
	Address string `json:"address"`
	Port    int    `json:"port"`
	Weight  int    `json:"weight"`
}

const (
	ModeTCP  = "tcp"
	ModeHTTP = "http"
	ModeUDP  = "udp" // TODO add BACKEND MODE TYPE
)

type BackendGroup struct {
	Name                     string     `json:"name"`
	SessionAffinityPolicy    string     `json:"session_affinity_policy"`
	SessionAffinityAttribute string     `json:"session_affinity_attribute"`
	Mode                     string     `json:"mode"`
	Backends                 []*Backend `json:"backends"`
}

type BackendService struct {
	ServiceName string `json:"service_name"`
	ServiceNs   string `json:"service_ns"`
	ServicePort int    `json:"service_port"`
	Weight      int    `json:"weight"`
}

type Rule struct {
	Config           *RuleConfig `json:"config,omitempty"`
	RuleID           string      `json:"rule_id"`
	Priority         int         `json:"priority"` // priority set by user
	Type             string      `json:"type"`
	Domain           string      `json:"domain"`
	URL              string      `json:"url"`
	DSLX             v1.DSLX     `json:"dslx"`
	EnableCORS       bool        `json:"enable_cors"`
	CORSAllowHeaders string      `json:"cors_allow_headers"`
	CORSAllowOrigin  string      `json:"cors_allow_origin"`
	BackendProtocol  string      `json:"backend_protocol"`
	RedirectURL      string      `json:"redirect_url"`
	RedirectCode     int         `json:"redirect_code"`
	VHost            string      `json:"vhost"`
	// CertificateName = namespace_secretname
	CertificateName string `json:"certificate_name"`
	RewriteBase     string `json:"rewrite_base"`
	RewriteTarget   string `json:"rewrite_target"`
	Description     string `json:"description"`

	SessionAffinityPolicy string            `json:"session_affinity_policy"`
	SessionAffinityAttr   string            `json:"session_affinity_attribute"`
	Services              []*BackendService `json:"services"` // 这条规则对应的后端服务

	BackendGroup *BackendGroup `json:"-"` // 这条规则对应的后端pod的ip
}

func (rl Rule) AllowNoAddr() bool {
	return rl.RedirectURL != ""
}

func (rl Rule) GetRawPriority() int {
	return rl.Priority
}

func (rl Rule) GetPriority() int {
	return rl.DSLX.Priority()
}

type RuleList []*Rule

type BackendGroups []*BackendGroup

func (bgs BackendGroups) Len() int {
	return len(bgs)
}

func (bgs BackendGroups) Swap(i, j int) {
	bgs[i], bgs[j] = bgs[j], bgs[i]
}

func (bgs BackendGroups) Less(i, j int) bool {
	return bgs[i].Name > bgs[j].Name
}

var (
	//ErrStandAlone will be returned when do something that is not allowed in stand-alone mode
	ErrStandAlone = errors.New("operation is not allowed in stand-alone mode")
)

func GetController(kd *driver.KubernetesDriver) (Controller, error) {
	switch config.Get("LB_TYPE") {
	case config.Nginx:
		return NewNginxController(kd), nil
	default:
		return nil, fmt.Errorf("Unsupport lb type %s only support nginx. Will support elb, slb, clb in the future", config.Get("LB_TYPE"))
	}
}
