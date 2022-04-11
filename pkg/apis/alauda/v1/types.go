package v1

// +kubebuilder:validation:Optional

import (
	"fmt"
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
type ALB2 struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ALB2Spec   `json:"spec"`
	Status ALB2Status `json:"status"`
}

type ALB2Spec struct {
	Address     string   `json:"address"`      // just for display in website
	BindAddress string   `json:"bind_address"` //deprecated
	Domains     []string `json:"domains"`      // deprecated
	IaasID      string   `json:"iaas_id"`      // deprecated
	Type        string   `json:"type"`
}

type ALB2Status struct {
	State     string `json:"state"`
	Reason    string `json:"reason"`
	ProbeTime int64  `json:"probeTime"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ALB2List struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ALB2 `json:"items"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:shortName=ft,scope=Namespaced
// +kubebuilder:subresource:status
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
)

type FrontendSpec struct {
	Port            int           `json:"port"` // port in service
	Protocol        FtProtocol    `json:"protocol"`
	ServiceGroup    *ServiceGroup `json:"serviceGroup,omitempty"`
	Source          *Source       `json:"source,omitempty"`
	CertificateName string        `json:"certificate_name"`
	BackendProtocol string        `json:"backendProtocol"`
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
	SessionAffinityPolicy    string    `json:"session_affinity_policy,omitempty"`
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
	Key    string     `json:"key,omitempty"`
}

type DSLX []DSLXTerm

type RuleSpec struct {
	Description string `json:"description"`
	Domain      string `json:"domain"`
	// +optional
	DSL              string        `json:"dsl"` // deprecated
	DSLX             DSLX          `json:"dslx"`
	Priority         int           `json:"priority"`
	ServiceGroup     *ServiceGroup `json:"serviceGroup,omitempty"`
	Source           *Source       `json:"source,omitempty"`
	Type             string        `json:"type"`
	URL              string        `json:"url"`
	CertificateName  string        `json:"certificate_name"`
	EnableCORS       bool          `json:"enableCORS"`
	CORSAllowHeaders string        `json:"corsAllowHeaders"`
	CORSAllowOrigin  string        `json:"corsAllowOrigin"`
	BackendProtocol  string        `json:"backendProtocol"`
	RedirectURL      string        `json:"redirectURL"`
	VHost            string        `json:"vhost"`
	RedirectCode     int           `json:"redirectCode"`
	// +optional
	RewriteBase string `json:"rewrite_base,omitempty"`
	// +optional
	RewriteTarget string `json:"rewrite_target,omitempty"`
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
		if term.Type == KEY_HOST {
			if term.Values[0][0] == OP_EQ || term.Values[0][0] == OP_IN {
				// exact host has bigger weight 50000, make sure concrete-host prioritize generic-host
				p += 50000
			} else if term.Values[0][0] == OP_ENDS_WITH {
				// generic host has smaller weight 10000
				p += 10000
			}
		} else if term.Type == KEY_URL {
			for _, item := range term.Values {
				if item[0] == OP_EQ {
					// EQ is more concrete than STARTS_WITH/REGEX, thus has bigger weight 2000
					p += 2000
				} else if item[0] == OP_STARTS_WITH {
					// STARTS_WITH is more concrete than REGEX, thus has bigger weight 1000
					p += 1000
				} else if item[0] == OP_REGEX {
					// REGEX is less concrete than EQ/STARTS_WITH, thus has smaller weight 500
					p += 500
				}
			}
		} else {
			p += 100 * len(term.Values)
		}
	}
	return p
}
