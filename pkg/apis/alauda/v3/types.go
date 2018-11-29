package v3

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient:nonNamespaced

// AlaudaLoadBalancer holds all the information in one crd
type AlaudaLoadBalancer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec AlaudaLoadBalancerSpec `json:"spec"`
}

type DomainInfo struct {
	Disabled bool   `json:"disabled"`
	Domain   string `json:"domain"`
	Type     string `json:"type"`
}

type Frontend struct {
	CertificateID   string `json:"certificate_id"`
	CertificateName string `json:"certificate_name"`
	ContainerPort   int    `json:"container_port"`
	LoadbalancerID  string `json:"load_balancer_id"`
	Port            int    `json:"port"`
	Protocol        string `json:"protocol"`
	Rules           []Rule `json:"rules"`
	ServiceID       string `json:"service_id"`
}

type Rule struct {
	CertificateID            string    `json:"certificate_id"`
	CertificateName          string    `json:"certificate_name"`
	Description              string    `json:"description"`
	Domain                   string    `json:"domain"`
	DSL                      string    `json:"dsl"`
	Priority                 int       `json:"priority"`
	RuleID                   string    `json:"rule_id"`
	Services                 []Service `json:"services"`
	SessionAffinityAttribute string    `json:"session_affinity_attribute"`
	SessionAffinityPolicy    string    `json:"session_affinity_policy"`
	Type                     string    `json:"type"`
	URL                      string    `json:"url"`
}

type Service struct {
	ContainerPort int    `json:"container_port"`
	ServiceID     string `json:"service_id"`
	Weight        int    `json:"weight"`
}

type AlaudaLoadBalancerSpec struct {
	Address     string       `json:"address"`
	BindAddress string       `json:"bind_address"`
	Domains     []DomainInfo `json:"domain_info"`
	Frontends   []Frontend   `json:"frontends"`
	IaasID      string       `json:"iaas_id"`
	Name        string       `json:"name"`
	Type        string       `json:"type"`
	Version     int          `json:"version"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type AlaudaLoadBalancerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []AlaudaLoadBalancer `json:"items"`
}
