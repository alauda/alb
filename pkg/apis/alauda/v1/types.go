package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ALB2 struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ALB2Spec `json:"spec"`
}

type ALB2Spec struct {
	Address     string   `json:"address"`
	BindAddress string   `json:"bind_address"`
	Domains     []string `json:"domains"`
	IaasID      string   `json:"iaas_id"`
	Type        string   `json:"type"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ALB2List struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ALB2 `json:"items"`
}

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type Frontend struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec FrontendSpec `json:"spec"`
}

type FrontendSpec struct {
	CertificateID   string        `json:"certificate_id"`
	CertificateName string        `json:"certificate_name"`
	Port            int           `json:"port"`
	Protocol        string        `json:"protocol"`
	ServiceGroup    *ServiceGroup `json:"serviceGroup,omitempty"`
	Source          *Source       `json:"source,omitempty"`
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
type Source struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Type      string `json:"type"`
}

type RuleSpec struct {
	Description  string        `json:"description"`
	Domain       string        `json:"domain"`
	DSL          string        `json:"dsl"`
	Priority     int           `json:"priority"`
	ServiceGroup *ServiceGroup `json:"serviceGroup,omitempty"`
	Source       *Source       `json:"source,omitempty"`
	Type         string        `json:"type"`
	URL          string        `json:"url"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type RuleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Rule `json:"items"`
}
