package types

import (
	albv1 "alauda.io/alb2/pkg/apis/alauda/v1"
	v1 "alauda.io/alb2/pkg/apis/alauda/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	auth_t "alauda.io/alb2/pkg/controller/ext/auth/types"
	keepalive_t "alauda.io/alb2/pkg/controller/ext/keepalive/types"
	otelt "alauda.io/alb2/pkg/controller/ext/otel/types"
	redirect_t "alauda.io/alb2/pkg/controller/ext/redirect/types"
	timeout_t "alauda.io/alb2/pkg/controller/ext/timeout/types"
	waft "alauda.io/alb2/pkg/controller/ext/waf/types"
	corev1 "k8s.io/api/core/v1"
)

type RefMap struct {
	ConfigMap map[client.ObjectKey]*corev1.ConfigMap
	Secret    map[client.ObjectKey]*corev1.Secret
}

type LoadBalancer struct {
	Labels      map[string]string
	Annotations map[string]string
	Name        string
	Address     string
	Type        string
	Version     int
	Frontends   []*Frontend
	Refs        RefMap
}

// 代表的是ft单独作为一个路由规则时的配置。比如l4的配置。l7 redirect或者默认转发的配置
type FtConf struct {
	Timeout   *timeout_t.TimeoutCr     `json:"timeout,omitempty"`
	Redirect  *redirect_t.RedirectCr   `json:"redirect,omitempty"`
	KeepAlive *keepalive_t.KeepAliveCr `json:"keepalive,omitempty"`
}

type Frontend struct {
	Labels          map[string]string `json:"-"`
	FtName          string            `json:"-"`        // ft name
	AlbName         string            `json:"alb_name"` // alb name
	Port            v1.PortNumber     `json:"port"`
	Protocol        v1.FtProtocol     `json:"protocol"` // ft 支持的协议 http/https/tcp/udp/grpc tcp 和 udp 代表 stream mode
	Config          FtConf            `json:"config"`
	Rules           RuleList          `json:"rules"`
	Services        []*BackendService `json:"services"`         // ft 默认后端路由组
	BackendProtocol string            `json:"backend_protocol"` // ft 默认后端路由组对应的协议
	BackendGroup    *BackendGroup     `json:"-"`                // ft 默认后端路由组对应的 endpoint 权重、均衡算法等相关信息
	CertificateName string            `json:"certificate_name"` // ft 默认证书
	Conflict        bool              `json:"-"`
}

type Domain struct {
	Domain   string `json:"domain"`
	Type     string `json:"type,omitempty"`
	Disabled bool   `json:"disabled"`
}

type Certificate struct {
	Cert string `json:"cert"`
	Key  string `json:"key"`
}

type CaCertificate struct {
	Cert string `json:"cert"`
}

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

type NgxPolicy struct {
	CertificateMap map[string]Certificate `json:"certificate_map"`
	Http           HttpPolicy             `json:"http"`
	Stream         StreamPolicy           `json:"stream"`
	SharedConfig   SharedExtPolicyConfig  `json:"config"`
	BackendGroup   []*BackendGroup        `json:"backend_group"`
}

type (
	Policies   []*Policy
	HttpPolicy struct {
		Tcp map[albv1.PortNumber]Policies `json:"tcp"`
	}
)

type SharedExtPolicyConfig map[string]RefBox

type StreamPolicy struct {
	Tcp map[albv1.PortNumber]Policies `json:"tcp"`
	Udp map[albv1.PortNumber]Policies `json:"udp"`
}

// keep it as same as rule
type Source struct {
	SourceType string `json:"source_type,omitempty"`
	SourceName string `json:"source_name,omitempty"`
	SourceNs   string `json:"source_ns,omitempty"`
}

type Policy struct {
	// match
	InternalDSL []interface{} `json:"internal_dsl,omitempty"` // dsl determine whether a request match this rule, same as rule.spec.dlsx

	PolicySortBean `json:"-"`

	Upstream        string `json:"upstream"`         // upstream_refs
	BackendProtocol string `json:"backend_protocol"` // set to variable $backend_protocol, used in proxy_pass $backend_protocol://http_backend; in nginx.conf

	// meta
	Rule      string `json:"rule"` // rule_refs the name of rule, corresponding with k8s rule cr
	Subsystem string `json:"subsystem"`
	Source    Source `json:"source"`

	LegacyExtInPolicy              // some legacy extension should migrate to the config field
	Config            PolicyExtCfg `json:"config"` // config or reference

	ToLocation *string `json:"to_location,omitempty"`

	Plugins []string `json:"plugins,omitempty"` // a list of lua module which enabled for this rule
}

type PolicySortBean struct {
	Priority        int `json:"-"` // priority set by user, used to sort policy which is rule's priority
	ComplexPriority int
	InternalDSLLen  int
}

func (p *PolicySortBean) MakeItMatchFirst() {
	p.Priority = -1
	p.ComplexPriority = 999999999
}

func (p *PolicySortBean) MakeItMatchLast() {
	p.ComplexPriority = -1 // default rule should have the minimum priority
	p.Priority = 999       // minimum number means higher priority
}

// rule cr/gateway cr => internal-rule => policy
// 一个internal rule 代表了一个转发规则的最小的*完整*的信息单元
// 最核心的有
// 1. match 描述请求和规则是否匹配
// 2. sortbean 描述这个规则在所有规则中的排序位置
// 3. upstream 转发到那个后端,这个后端的转发相关的配置

type InternalRule struct {
	RuleMeta
	RuleMatch
	RuleCert
	Config RuleExt
	RuleUpstream
}

type RuleMeta struct {
	Type     string     `json:"type"`    // 这个internal rule是从那个结构转换来的，目前有rule和gateway
	RuleID   string     `json:"rule_id"` // rule的标示,对alb-rule 是alb-rule的name，对gateway api route是这个route的唯一路径
	Source   *v1.Source `json:"source,omitempty"`
	Priority int        `json:"-"` // priority set by user, used to sort policy which is rule's priority
}

type RuleMatch struct { // 和匹配规则
	DSLX v1.DSLX `json:"-"`
}

type RuleCert struct { // 和证书有关的配置
	// CertificateName = namespace_secretName
	CertificateName string `json:"certificate_name"` // cert_ref
	Domain          string `json:"domain"`           // used to fetch cert.
}

// 直接放在rule.spec 而不是rule.spec.config
type Vhost struct {
	VHost string `json:"vhost"` // ext vhost
}

type RewriteConf struct {
	URL                  string  `json:"url"`                              // for rewrite // alb-rule
	RewriteBase          string  `json:"rewrite_base"`                     // alb-rule
	RewriteTarget        string  `json:"rewrite_target"`                   // alb-rule
	RewritePrefixMatch   *string `json:"rewrite_prefix_match,omitempty"`   // gatewayapi-httproute
	RewriteReplacePrefix *string `json:"rewrite_replace_prefix,omitempty"` // gatewayapi-httproute
}

type Cors struct { // ext cors
	EnableCORS       bool   `json:"enable_cors"`
	CORSAllowHeaders string `json:"cors_allow_headers"`
	CORSAllowOrigin  string `json:"cors_allow_origin"`
}

type RuleExt struct { // 不同的扩展的配置
	Rewrite         *RewriteConf
	Cors            *Cors
	Vhost           *Vhost
	Timeout         *timeout_t.TimeoutCr   `json:"timeout,omitempty"`
	RewriteResponse *RewriteResponseConfig `json:"rewrite_response,omitempty"`
	RewriteRequest  *RewriteRequestConfig  `json:"rewrite_request,omitempty"`
	Otel            *otelt.OtelConf        `json:"otel,omitempty"`
	Redirect        *redirect_t.RedirectCr `json:"redirect,omitempty"`
	Waf             *waft.WafInternal
	Auth            *auth_t.AuthCr
	Source          ConfigSource
}

type (
	ConfigSource  map[PolicyExtKind]string
	PolicyExtKind string
)

// keep this as same as policy_ext json annotation
const (
	Rewrite         PolicyExtKind = "rewrite"
	CORS            PolicyExtKind = "cors"
	RewriteRequest  PolicyExtKind = "rewrite_request"
	RewriteResponse PolicyExtKind = "rewrite_response"
	Timeout         PolicyExtKind = "timeout"
	Redirect        PolicyExtKind = "redirect"
	Otel            PolicyExtKind = "otel"
	Waf             PolicyExtKind = "waf"
	Auth            PolicyExtKind = "auth"
)

type PolicyExt struct {
	RewriteResponse *RewriteResponseConfig `json:"rewrite_response,omitempty"`
	RewriteRequest  *RewriteRequestConfig  `json:"rewrite_request,omitempty"`
	Timeout         *timeout_t.TimeoutCr   `json:"timeout,omitempty"`
	Otel            *otelt.OtelConf        `json:"otel,omitempty"`
	Auth            *auth_t.AuthPolicy     `json:"auth,omitempty"`
	Redirect        *redirect_t.RedirectCr `json:"redirect,omitempty"`
	Source          string                 `json:"-"`
}

type PolicyExtCfg struct {
	PolicyExt
	Refs map[PolicyExtKind]string `json:"refs,omitempty"`
}

// TODO use code-gen
func (p *PolicyExt) Clean(key PolicyExtKind) {
	if key == RewriteRequest {
		p.RewriteRequest = nil
	}
	if key == RewriteResponse {
		p.RewriteResponse = nil
	}
	if key == Timeout {
		p.Timeout = nil
	}
	if key == Otel {
		p.Otel = nil
	}
	if key == Auth {
		p.Auth = nil
	}
	if key == Redirect {
		p.Redirect = nil
	}
}

// 将其转换为map方便后续去重
// TODO use code-gen
func (p PolicyExtCfg) ToMaps() PolicyExtMap {
	m := PolicyExtMap{}
	if p.RewriteResponse != nil {
		m[RewriteResponse] = &PolicyExt{RewriteResponse: p.RewriteResponse, Source: p.Refs[RewriteResponse]}
	}
	if p.RewriteRequest != nil {
		m[RewriteRequest] = &PolicyExt{RewriteRequest: p.RewriteRequest, Source: p.Refs[RewriteRequest]}
	}
	if p.Timeout != nil {
		m[Timeout] = &PolicyExt{Timeout: p.Timeout, Source: p.Refs[Timeout]}
	}
	if p.Otel != nil {
		m[Otel] = &PolicyExt{Otel: p.Otel, Source: p.Refs[Otel]}
	}
	if p.Auth != nil {
		m[Auth] = &PolicyExt{Auth: p.Auth, Source: p.Refs[Auth]}
	}
	if p.Redirect != nil {
		m[Redirect] = &PolicyExt{Redirect: p.Redirect, Source: p.Refs[Redirect]}
	}
	return m
}

type PolicyExtMap = map[PolicyExtKind]*PolicyExt

type LegacyExtInPolicy struct {
	RewriteConf
	Cors
	Vhost
}

type RuleUpstream struct { // 不同的扩展的配置
	BackendProtocol       string            `json:"backend_protocol"`           // set to variable $backend_protocol, used in proxy_pass $backend_protocol://http_backend; in nginx.conf
	SessionAffinityPolicy string            `json:"session_affinity_policy"`    // will be set in upstream config
	SessionAffinityAttr   string            `json:"session_affinity_attribute"` // will be set in upstream config
	Services              []*BackendService `json:"services"`                   // 这条规则对应的后端服务
	BackendGroup          *BackendGroup     `json:"-"`                          // 这条规则对应的后端 pod 的 ip
}

// policy.json http match rule config

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

type RefBox struct {
	Hash string        `json:"-"`
	Type PolicyExtKind `json:"type"`
	Note *string       `json:"note,omitempty"`
	PolicyExt
}
