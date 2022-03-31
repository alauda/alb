package types

import (
	"encoding/json"
	"fmt"

	gatewayPolicy "alauda.io/alb2/pkg/apis/alauda/gateway/v1alpha1"
	v1 "alauda.io/alb2/pkg/apis/alauda/v1"
)

const (
	SubsystemHTTP   = "http"
	SubsystemStream = "stream"

	PolicySIPHash = "sip-hash"
	PolicyCookie  = "cookie"
)

var LastConfig = ""
var LastFailure = false

type Domain struct {
	Domain   string `json:"domain"`
	Type     string `json:"type,omitempty"`
	Disabled bool   `json:"disabled"`
}

type LoadBalancer struct {
	Labels    map[string]string `json:"-"`
	Name      string            `json:"name"`
	Address   string            `json:"address"`
	Type      string            `json:"type"`
	Version   int               `json:"version"`
	Frontends []*Frontend       `json:"frontends"`
	TweakHash string            `json:"-"`
}

func (lb *LoadBalancer) String() string {
	r, err := json.Marshal(lb)
	if err != nil {
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
	FtName          string            `json:"-"`        // ft name
	AlbName         string            `json:"alb_name"` // alb name
	Port            int               `json:"port"`
	Protocol        v1.FtProtocol     `json:"protocol"` // ft的协议 http/https/tcp/udp tcp和udp代表stream mode
	Rules           RuleList          `json:"rules"`
	Services        []*BackendService `json:"services"`         // ft的默认后端路由组
	BackendProtocol string            `json:"backend_protocol"` // 这个默认后端路由组的协议
	BackendGroup    *BackendGroup     `json:"-"`                // 默认后端路由的pod地址 在
	CertificateName string            `json:"certificate_name"` // ft的默认证书
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

func (b *Backend) Eq(other *Backend) bool {
	return b.Address == other.Address && b.Port == other.Port && b.Weight == other.Weight
}

type Backends []*Backend

func (b Backend) String() string {
	return fmt.Sprintf("%v-%v-%v", b.Address, b.Port, b.Weight)
}

func (bs Backends) Len() int {
	return len(bs)
}

func (bs Backends) Less(i, j int) bool {
	return bs[i].String() < bs[j].String()
}
func (bs Backends) Swap(i, j int) bool {
	return bs[i].String() < bs[j].String()
}

func (bs Backends) Eq(other Backends) bool {
	if len(bs) != len(other) {
		return false
	}
	for i := range bs {
		if !bs[i].Eq(other[i]) {
			return false
		}
	}
	return true
}

const (
	ModeTCP  = "tcp"
	ModeHTTP = "http"
	ModeUDP  = "udp"
)

func FtProtocolToBackendMode(protocol v1.FtProtocol) string {
	switch protocol {
	case v1.FtProtocolTCP:
		return ModeTCP
	case v1.FtProtocolUDP:
		return ModeUDP
	case v1.FtProtocolHTTP:
		return ModeHTTP
	case v1.FtProtocolHTTPS:
		return ModeHTTP
	}
	return ""
}

const (
	RuleTypeIngress = "ingress"
	RuleTypeGateway = "gateway"
)

type BackendGroup struct {
	Name                     string   `json:"name"`
	SessionAffinityPolicy    string   `json:"session_affinity_policy"`
	SessionAffinityAttribute string   `json:"session_affinity_attribute"`
	Mode                     string   `json:"mode"`
	Backends                 Backends `json:"backends"`
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
	Domain           string      `json:"domain"` // used to fetch cert.
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

func (bg BackendGroup) Eq(other BackendGroup) bool {
	// bg equal other
	return bg.Name == other.Name &&
		bg.Mode == other.Mode &&
		bg.SessionAffinityAttribute == other.SessionAffinityAttribute &&
		bg.SessionAffinityPolicy == other.SessionAffinityPolicy &&
		bg.Backends.Eq(other.Backends)
}

type RuleConfig struct {
	RewriteResponse *RewriteResponseConfig             `json:"rewrite_response,omitempty"`
	Timeout         *gatewayPolicy.TimeoutPolicyConfig `json:"timeout,omitempty"`
}

type RewriteResponseConfig struct {
	Headers        map[string]string `json:"headers,omitempty"`
	HeadersDelete  map[string]string `json:"headers_delete,omitempty"`
	HeadersUpdate  map[string]string `json:"headers_update,omitempty"`
	HeadersDefault map[string]string `json:"headers_default,omitempty"`
}

func (r RewriteResponseConfig) IsEmpty() bool {
	return len(r.Headers) == 0
}

func (r RuleConfig) ToJsonString() (string, error) {
	ret, err := json.Marshal(&r)
	return string(ret), err
}

func (r RuleConfig) IsEmpty() bool {
	if r.RewriteResponse != nil && !r.RewriteResponse.IsEmpty() {
		return false
	}
	return true
}
