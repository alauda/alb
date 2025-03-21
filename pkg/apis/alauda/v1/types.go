package v1

// +kubebuilder:validation:Optional

import (
	"bytes"
	"encoding/json"
	"fmt"

	"alauda.io/alb2/pkg/apis/alauda/shared"
	keepalive_t "alauda.io/alb2/pkg/controller/ext/keepalive/types"
	otelt "alauda.io/alb2/pkg/controller/ext/otel/types"
	redirect_t "alauda.io/alb2/pkg/controller/ext/redirect/types"
	waft "alauda.io/alb2/pkg/controller/ext/waf/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ALB2Kind     = "ALB2"
	FrontendKind = "Frontend"
	RuleKind     = "Rule"
)

const (
	KEY_HOST = "HOST"
	KEY_URL  = "URL"

	OP_EQ        = "EQ"
	OP_IN        = "IN"
	OP_ENDS_WITH = "ENDS_WITH"

	OP_STARTS_WITH = "STARTS_WITH"
	OP_REGEX       = "REGEX"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:singular=alaudaloadbalancer2,path=alaudaloadbalancer2,shortName=alb2,scope=Namespaced
// +kubebuilder:subresource:status
// +kubebuilder:deprecatedversion:warning="alb2.v1.crd is deprecated,use alb2.v2beta1.crd instead."
type ALB2 struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ALB2Spec   `json:"spec"`
	Status ALB2Status `json:"status"`
}

type ALB2Spec struct {
	// address is only used to display at front-end.
	Address string `json:"address"` // just for display in website
	// bind_address is deprecated, default ""
	BindAddress string `json:"bind_address"` // deprecated
	// domains is deprecated, default []
	Domains []string `json:"domains"` // deprecated
	// iaas_id is deprecated, default ""
	IaasID string `json:"iaas_id"` // deprecated
	// type defines the loadbalancer alb2 uses, now only support nginx
	// +kubebuilder:validation:Enum=nginx
	Type string `json:"type"`
}

type ALB2Status struct {
	// state defines the status of alb2, the possible values are ready/warning
	// state:ready means ok
	// state:warning can be caused by port conflict in alb2
	State string `json:"state"`
	// reason defines the possible cause of alb2 state change
	Reason    string `json:"reason"`
	ProbeTime int64  `json:"probeTime"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ALB2List struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ALB2 `json:"items"`
}

// PortNumber defines a network port
//
// +kubebuilder:validation:Minimum=1
// +kubebuilder:validation:Maximum=65535
type PortNumber int32

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:shortName=ft,scope=Namespaced
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:JSONPath=".spec.protocol",description="protocol",name="protocol",type=string
type Frontend struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FrontendSpec   `json:"spec"`
	Status FrontendStatus `json:"status"`
}

type FtProtocol string

const (
	FtProtocolTCP   FtProtocol = "tcp"
	FtProtocolUDP   FtProtocol = "udp"
	FtProtocolHTTP  FtProtocol = "http"
	FtProtocolHTTPS FtProtocol = "https"
	FtProtocolgRPC  FtProtocol = "grpc"
)

type FrontendSpec struct {
	Port     PortNumber `json:"port"`
	Protocol FtProtocol `json:"protocol"`
	// +optional
	ServiceGroup *ServiceGroup `json:"serviceGroup,omitempty"`
	// +optional
	Source *Source `json:"source,omitempty"`
	// certificate_name defines certificate used for https frontend
	CertificateName string `json:"certificate_name"`
	// backendProtocol defines protocol used by backend servers, it could be https/http/grpc
	BackendProtocol string    `json:"backendProtocol"`
	Config          *FTConfig `json:"config,omitempty"`
}

type FrontendStatus struct {
	Instances map[string]Instance `json:"instances"`
}

type Instance struct {
	Conflict  bool  `json:"conflict"`
	ProbeTime int64 `json:"probeTime"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type FrontendList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Frontend `json:"items"`
}

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:shortName=rl,scope=Namespaced
// +kubebuilder:printcolumn:JSONPath=".spec.priority",description="priority",name="priority",type=integer
// +kubebuilder:printcolumn:JSONPath=".spec.dsl",description="description of match condition",name="matches",type=string
// +kubebuilder:printcolumn:JSONPath=".spec.source.type",description="source type of this rule",name="source-type",type=string
// +kubebuilder:printcolumn:JSONPath=".spec.source.namespace",description="source ns of this rule",name="source-ns",type=string
// +kubebuilder:printcolumn:JSONPath=".spec.source.name",description="source name of this rule",name="source-name",type=string

type Rule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec RuleSpec `json:"spec"`
}

type Service struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Port      int    `json:"port"` // port in service, not port in pod.
	Weight    int    `json:"weight"`
}

type ServiceGroup struct {
	// +optional
	SessionAffinityPolicy string `json:"session_affinity_policy,omitempty"`
	// +optional
	SessionAffinityAttribute string    `json:"session_affinity_attribute,omitempty"`
	Services                 []Service `json:"services"`
}

// Source is where the frontend or rule came from.
// It's type can be "bind" for those created for service annotations.
// And be "ingress" for those created for ingress resource
type Source struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Type      string `json:"type"`
}

type DSLXTerm struct {
	Values [][]string `json:"values"`
	Type   string     `json:"type"`
	// +optional
	Key string `json:"key,omitempty"`
}

type DSLX []DSLXTerm

func (d DSLX) ToSearchableString() string {
	return fmt.Sprintf("%v", d)
}

type RuleSpec struct {
	// type is deprecated
	Type        string `json:"type"`
	Description string `json:"description"`
	Domain      string `json:"domain"`
	// +optional
	// used for searching on the UI interface
	DSL string `json:"dsl"`
	// +optional
	ServiceGroup *ServiceGroup `json:"serviceGroup,omitempty"`
	// certificate_name defines certificate used with specified hostname in rule at https frontend
	CertificateName string `json:"certificate_name"`

	Config *RuleConfigInCr `json:"config,omitempty"`

	// 下面的部分和rule 和 policy 是一模一样的

	// priority ranges from [1,10], if multiple rules match, less value prioritize
	Priority int `json:"priority"`
	// dslx defines the matching criteria
	DSLX DSLX `json:"dslx"`
	// +optional
	URL string `json:"url"`
	// +optional
	RewriteBase string `json:"rewrite_base,omitempty"`
	// +optional
	RewriteTarget string `json:"rewrite_target,omitempty"`
	// enableCORS is the switch whether enable cross domain, when EnableCORS is false, alb2 transports information to backend servers which determine whether allow cross-domain
	EnableCORS bool `json:"enableCORS"`
	// corsAllowHeaders defines the headers allowed by cors when enableCORS is true
	CORSAllowHeaders string `json:"corsAllowHeaders"`
	// corsAllowOrigin defines the origin allowed by cors when enableCORS is true
	CORSAllowOrigin string `json:"corsAllowOrigin"`
	// backendProtocol defines protocol used by backend servers, it could be https/http/grpc
	BackendProtocol string `json:"backendProtocol"`
	// Deprecated: use config.redirect.redirect_url
	RedirectURL string `json:"redirectURL"`
	// vhost allows user to override the request Host
	VHost string `json:"vhost"`
	// Deprecated: use config.redirect.redirect_code. could be 301(Permanent Redirect)/302(Temporal Redirect), default 0
	RedirectCode int `json:"redirectCode"`
	// +optional
	// source is where the frontend or rule came from. It's type can be "bind" for those created for service annotations. And carries information about ingress when rule is generalized by ingress
	Source *Source `json:"source,omitempty"`
}

type FTConfig struct {
	shared.SharedCr `json:",inline"`
	Redirect        *redirect_t.RedirectCr   `json:"redirect,omitempty"`
	KeepAlive       *keepalive_t.KeepAliveCr `json:"keepalive,omitempty"`
}

type RuleConfigInCr struct {
	shared.SharedCr `json:",inline"`
	Redirect        *redirect_t.RedirectCr `json:"redirect,omitempty"`
}

func (r *Rule) GetWaf() *waft.WafCrConf {
	if r.Spec.Config == nil {
		return nil
	}
	if r.Spec.Config.ModeSecurity == nil {
		return nil
	}
	return r.Spec.Config.ModeSecurity
}

func (r *Rule) GetOtel() *otelt.OtelCrConf {
	if r.Spec.Config == nil {
		return nil
	}
	if r.Spec.Config.Otel == nil {
		return nil
	}
	return r.Spec.Config.Otel
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type RuleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Rule `json:"items"`
}

func (s Service) String() string {
	return fmt.Sprintf("%s-%s-%d", s.Namespace, s.Name, s.Port)
}

func (s Service) ServiceID() string {
	return fmt.Sprintf("%s.%s", s.Name, s.Namespace)
}

func (s Service) Is(ns, name string, port int) bool {
	if s.Namespace == ns &&
		s.Name == name &&
		s.Port == port {
		return true
	}
	return false
}

func (dslx DSLX) Priority() int {
	var p int
	for _, term := range dslx {
		switch term.Type {
		case KEY_HOST:
			switch term.Values[0][0] {
			case OP_EQ, OP_IN:
				// exact host has bigger weight 50000, make sure concrete-host prioritize generic-host
				p += 50000
			case OP_ENDS_WITH:
				// generic host has smaller weight 10000
				p += 10000
			}
		case KEY_URL:
			for _, item := range term.Values {
				op := item[0]
				if op == OP_EQ {
					// EQ is more concrete than STARTS_WITH/REGEX, thus has bigger weight 2000
					p += 2000
				}
				if op == OP_STARTS_WITH || op == OP_REGEX {
					val := item[1]
					p += 1000
					p += len(val)
					if op == OP_STARTS_WITH {
						p += 2 // STARTS_WITH /abc == REGEX /abc.*
					}
				}
			}
		default:
			p += 100 * len(term.Values)
		}
	}
	return p
}

// converting rules to deterministic strings,since that we could hash/diff rule spec.
func (r *RuleSpec) Identity() string {
	var b bytes.Buffer
	b.WriteString(r.Domain)
	b.WriteString(r.DSL)
	b.WriteString(fmt.Sprintf("%v", r.DSLX))
	b.WriteString(fmt.Sprintf("%v", r.Priority))
	b.WriteString(fmt.Sprintf("%v", r.ServiceGroup))
	b.WriteString(fmt.Sprintf("%v", r.Source))
	b.WriteString(r.Type)
	b.WriteString(r.URL)
	b.WriteString(r.CertificateName)
	b.WriteString(fmt.Sprintf("%v", r.EnableCORS))
	b.WriteString(r.CORSAllowHeaders)
	b.WriteString(r.BackendProtocol)
	b.WriteString(r.RedirectURL)
	b.WriteString(r.CORSAllowHeaders)
	b.WriteString(r.VHost)
	b.WriteString(fmt.Sprintf("%v", r.RedirectCode))
	b.WriteString(r.RewriteBase)
	b.WriteString(r.RewriteTarget)
	conf, _ := json.Marshal(r.Config) //nolint:errcheck
	b.WriteString(string(conf))
	return b.String()
}
