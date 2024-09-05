package types

import (
	"encoding/json"
	"fmt"

	gatewayPolicy "alauda.io/alb2/pkg/apis/alauda/gateway/v1alpha1"
	v1 "alauda.io/alb2/pkg/apis/alauda/v1"
	otelt "alauda.io/alb2/pkg/controller/ext/otel/types"
	waft "alauda.io/alb2/pkg/controller/ext/waf/types"
	corev1 "k8s.io/api/core/v1"
)

const (
	SubsystemHTTP   = "http"
	SubsystemStream = "stream"

	PolicySIPHash = "sip-hash"
	PolicyCookie  = "cookie"

	CaCert = "ca.crt"
)

var (
	LastConfig  = ""
	LastFailure = false
)

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
	CmRefs    map[string]*corev1.ConfigMap
}

type Certificate struct {
	Cert string `json:"cert"`
	Key  string `json:"key"`
}

type CaCertificate struct {
	Cert string `json:"cert"`
}

type Frontend struct {
	Labels          map[string]string `json:"-"`
	FtName          string            `json:"-"`        // ft name
	AlbName         string            `json:"alb_name"` // alb name
	Port            v1.PortNumber     `json:"port"`
	Protocol        v1.FtProtocol     `json:"protocol"` // ft 支持的协议 http/https/tcp/udp/grpc tcp 和 udp 代表 stream mode
	Rules           RuleList          `json:"rules"`
	Services        []*BackendService `json:"services"`         // ft 默认后端路由组
	BackendProtocol string            `json:"backend_protocol"` // ft 默认后端路由组对应的协议
	BackendGroup    *BackendGroup     `json:"-"`                // ft 默认后端路由组对应的 endpoint 权重、均衡算法等相关信息
	CertificateName string            `json:"certificate_name"` // ft 默认证书
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

func (ft *Frontend) IsGRPCMode() bool {
	return ft.Protocol == v1.FtProtocolgRPC
}

func (ft *Frontend) IsValidProtocol() bool {
	return ft.Protocol == v1.FtProtocolHTTP ||
		ft.Protocol == v1.FtProtocolHTTPS ||
		ft.Protocol == v1.FtProtocolTCP ||
		ft.Protocol == v1.FtProtocolUDP ||
		ft.Protocol == v1.FtProtocolgRPC
}

func (b *Backend) Eq(other *Backend) bool {
	return b.Address == other.Address && b.Port == other.Port && b.Weight == other.Weight
}

func (b Backend) String() string {
	return fmt.Sprintf("%v-%v-%v", b.Address, b.Port, b.Weight)
}

func (bs Backends) Len() int {
	return len(bs)
}

func (bs Backends) Less(i, j int) bool {
	return bs[i].String() < bs[j].String()
}

func (bs Backends) Swap(i, j int) {
	bs[i], bs[j] = bs[j], bs[i]
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
	ModegRPC = "grpc"
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
	case v1.FtProtocolgRPC:
		return ModegRPC
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

type Backends []*Backend

type Backend struct {
	Address           string  `json:"address"`
	Pod               string  `json:"-"`
	FromOtherClusters bool    `json:"otherclusters"`
	Port              int     `json:"port"`
	Svc               string  `json:"svc"`
	Ns                string  `json:"ns"`
	Weight            int     `json:"weight"`
	Protocol          string  `json:"-"`
	AppProtocol       *string `json:"-"`
}

type BackendService struct {
	ServiceName string  `json:"service_name"`
	ServiceNs   string  `json:"service_ns"`
	ServicePort int     `json:"service_port"`
	Protocol    string  `json:"protocol"`
	AppProtocol *string `json:"app_protocol"`
	Weight      int     `json:"weight"`
}

// rule cr/gateway cr => rule => policy
type Rule struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Domain      string `json:"domain"` // used to fetch cert.

	// CertificateName = namespace_secretName
	CertificateName string `json:"certificate_name"`

	SessionAffinityPolicy string            `json:"session_affinity_policy"`
	SessionAffinityAttr   string            `json:"session_affinity_attribute"`
	Services              []*BackendService `json:"services"` // 这条规则对应的后端服务
	BackendGroup          *BackendGroup     `json:"-"`        // 这条规则对应的后端 pod 的 ip

	RuleID string              `json:"rule_id"`
	Config *RuleConfigInPolicy `json:"config,omitempty"`

	Waf *waft.WafInRule `json:"waf"` // waf not need to be in policy.json
	SameInRuleCr
	SameInPolicy
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

// policy.json http match rule config
type RuleConfigInPolicy struct {
	RewriteResponse *RewriteResponseConfig             `json:"rewrite_response,omitempty"`
	RewriteRequest  *RewriteRequestConfig              `json:"rewrite_request,omitempty"`
	Timeout         *gatewayPolicy.TimeoutPolicyConfig `json:"timeout,omitempty"`
	Otel            *otelt.OtelInPolicy                `json:"otel,omitempty"`
}

type RewriteResponseConfig struct {
	Headers       map[string]string   `json:"headers,omitempty"`
	HeadersRemove []string            `json:"headers_remove,omitempty"`
	HeadersAdd    map[string][]string `json:"headers_add,omitempty"`
}

type RewriteRequestConfig struct {
	Headers       map[string]string   `json:"headers,omitempty"`
	HeadersVar    map[string]string   `json:"headers_var,omitempty"`
	HeadersRemove []string            `json:"headers_remove,omitempty"`
	HeadersAdd    map[string][]string `json:"headers_add,omitempty"`
	HeadersAddVar map[string][]string `json:"headers_add_var,omitempty"`
}

func (r RewriteResponseConfig) IsEmpty() bool {
	return len(r.Headers) == 0
}

func (r RuleConfigInPolicy) ToJsonString() (string, error) {
	ret, err := json.Marshal(&r)
	return string(ret), err
}
