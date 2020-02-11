package v1

import (
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ALB2Kind     = "ALB2"
	FrontendKind = "Frontend"
	RuleKind     = "Rule"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ALB2 struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ALB2Spec   `json:"spec"`
	Status ALB2Status `json:"status"`
}

type ALB2Spec struct {
	Address     string   `json:"address"`
	BindAddress string   `json:"bind_address"`
	Domains     []string `json:"domains"`
	IaasID      string   `json:"iaas_id"`
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

type Frontend struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FrontendSpec   `json:"spec"`
	Status FrontendStatus `json:"status"`
}

type FrontendSpec struct {
	Port            int           `json:"port"`
	Protocol        string        `json:"protocol"`
	ServiceGroup    *ServiceGroup `json:"serviceGroup,omitempty"`
	Source          *Source       `json:"source,omitempty"`
	CertificateName string        `json:"certificate_name"`
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

type Rule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec RuleSpec `json:"spec"`
}

type Service struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Port      int    `json:"port"`
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
	Description     string        `json:"description"`
	Domain          string        `json:"domain"`
	DSL             string        `json:"dsl"`
	DSLX            DSLX          `json:"dslx"`
	Priority        int           `json:"priority"`
	ServiceGroup    *ServiceGroup `json:"serviceGroup,omitempty"`
	Source          *Source       `json:"source,omitempty"`
	Type            string        `json:"type"`
	URL             string        `json:"url"`
	CertificateName string        `json:"certificate_name"`
	EnableCORS      bool          `json:"enableCORS"`
	BackendProtocol string        `json:"backendProtocol"`
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
