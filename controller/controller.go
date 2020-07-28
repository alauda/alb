package controller

import (
	v1 "alauda.io/alb2/pkg/apis/alauda/v1"
	"alauda.io/alb2/utils"
	"encoding/json"
	"errors"
	"fmt"
	"k8s.io/klog"
	"os/exec"
	"strings"

	"alauda.io/alb2/config"
	"alauda.io/alb2/driver"
)

const (
	ProtocolHTTP  = "http"
	ProtocolHTTPS = "https"
	ProtocolTCP   = "tcp"
	ProtocolUDP   = "udp"

	SubsystemHTTP   = "http"
	SubsystemStream = "stream"
	SubsystemDgram  = "dgram"

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
	TweakHash      string      `json:"-"`
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
	RawName         string            `json:"-"`
	LoadBalancerID  string            `json:"load_balancer_id"`
	Port            int               `json:"port"`
	Protocol        string            `json:"protocol"`
	Rules           RuleList          `json:"rules"`
	Services        []*BackendService `json:"services"`
	BackendProtocol string            `json:"backend_protocol"`

	BackendGroup    *BackendGroup `json:"-"`
	CertificateName string        `json:"certificate_name"`
}

func (ft *Frontend) String() string {
	return fmt.Sprintf("%s-%d-%s", ft.LoadBalancerID, ft.Port, ft.Protocol)
}

type Backend struct {
	Address string `json:"address"`
	Port    int    `json:"port"`
	Weight  int    `json:"weight"`
}

const (
	ModeTCP  = "tcp"
	ModeHTTP = "http"
)

type BackendGroup struct {
	Name                     string     `json:"name"`
	SessionAffinityPolicy    string     `json:"session_affinity_policy"`
	SessionAffinityAttribute string     `json:"session_affinity_attribute"`
	Mode                     string     `json:"mode"`
	Backends                 []*Backend `json:"backends"`
}

type BackendService struct {
	ServiceID     string `json:"service_id"`
	ContainerPort int    `json:"container_port"`
	Weight        int    `json:"weight"`
}

type Rule struct {
	RuleID          string  `json:"rule_id"`
	Priority        int     `json:"priority"`
	Type            string  `json:"type"`
	Domain          string  `json:"domain"`
	URL             string  `json:"url"`
	DSL             string  `json:"dsl"`
	DSLX            v1.DSLX `json:"dslx"`
	EnableCORS      bool    `json:"enable_cors"`
	BackendProtocol string  `json:"backend_protocol"`
	RedirectURL     string  `json:"redirect_url"`
	RedirectCode    int     `json:"redirect_code"`
	VHost           string  `json:"vhost"`
	// CertificateName = namespace_secretname
	CertificateName       string            `json:"certificate_name"`
	RewriteTarget         string            `json:"rewrite_target"`
	Description           string            `json:"description"`
	SessionAffinityPolicy string            `json:"session_affinity_policy"`
	SessionAffinityAttr   string            `json:"session_affinity_attribute"`
	Services              []*BackendService `json:"services"`

	BackendGroup *BackendGroup `json:"-"`
}

func (rl Rule) FillupDSL() {
	if rl.DSL == "" && (rl.Domain != "" || rl.URL != "") {
		klog.Info("transfer rl to dsl")
		if rl.Domain != "" && rl.URL != "" {
			if strings.HasPrefix(rl.URL, "^") {
				rl.DSL = fmt.Sprintf("(AND (EQ HOST %s) (REGEX URL %s))", rl.Domain, rl.URL)
			} else {
				rl.DSL = fmt.Sprintf("(AND (EQ HOST %s) (STARTS_WITH URL %s))", rl.Domain, rl.URL)
			}
		} else {
			if rl.Domain != "" {
				rl.DSL = fmt.Sprintf("(EQ HOST %s)", rl.Domain)
			} else {
				if strings.HasPrefix(rl.URL, "^") {
					rl.DSL = fmt.Sprintf("(REGEX URL %s)", rl.URL)
				} else {
					rl.DSL = fmt.Sprintf("(STARTS_WITH URL %s)", rl.URL)
				}
			}
		}
		if rl.DSL != "" && rl.DSLX == nil {
			dslx, err := utils.DSL2DSLX(rl.DSL)
			if err != nil {
				klog.Warning(err)
			} else {
				rl.DSLX = dslx
			}
		}
	}
}

func (rl Rule) GetPriority() int {
	var (
		dslx v1.DSLX
		err  error
	)
	// rl.Priority is not used in acp, ignore
	//if rl.Priority != 0 {
	//	return rl.Priority
	//}
	if rl.DSLX != nil {
		dslx = rl.DSLX
	} else {
		dslx, err = utils.DSL2DSLX(rl.DSL)
		if err != nil {
			return len(rl.DSL)
		}
	}

	return dslx.Priority() + len(rl.DSL)
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

type Config struct {
	Name             string
	Address          string
	BindAddress      string
	LoadBalancerID   string
	Frontends        map[int]*Frontend
	BackendGroup     BackendGroups
	CertificateMap   map[string]Certificate
	TweakHash        string
	EnablePrometheus bool
	EnableIPV6       bool
}

var (
	//ErrStandAlone will be return if do something that not allowed in stand mode
	ErrStandAlone = errors.New("operation is not allowed in stand alone mode")
)

func GetController(kd *driver.KubernetesDriver) (Controller, error) {
	switch config.Get("LB_TYPE") {
	case config.Nginx:
		return NewNginxController(kd), nil
	default:
		return nil, fmt.Errorf("Unsupport lb type %s only support nginx. Will support elb, slb, clb in the future", config.Get("LB_TYPE"))
	}
}
