package modules

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	//ProtoHTTP is the protocol of http frontend
	ProtoHTTP  = "http"
	ProtoHTTPS = "https"
	ProtoTCP   = "tcp"
	ProtoUDP   = "udp"
)

const (
	TypeBind    = "bind"
	TypeIngress = "ingress"
)

// SourceInfo is where the frontend or rule came from.
// It's type can be "bind" for those created for service annotations.
// And be "ingress" for those created for ingress resource
type SourceInfo struct {
	Type      string `json:"type"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

type Alb2Resource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	Spec              Alb2Spec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
}

type Alb2Spec struct {
	Address        string   `json:"address"`
	BindAddress    string   `json:"bind_address"`
	LoadBalancerID string   `json:"iaas_id"`
	Type           string   `json:"type"`
	Domains        []string `json:"domains"`
}

type FrontendList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	Items           []*FrontendResource `json:"items" protobuf:"bytes,2,rep,name=items"`
}

type FrontendResource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	Spec              FrontendSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
}

type FrontendSpec struct {
	Port            int            `json:"port"`
	Protocol        string         `json:"protocol"`
	CertificateID   string         `json:"certificate_id"`
	CertificateName string         `json:"certificate_name"`
	ServiceGroup    *ServicceGroup `json:"serviceGroup,omitempty"`
	Source          *SourceInfo    `json:"source,omitempty"`
}

type RuleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	Items           []*RuleResource `json:"items" protobuf:"bytes,2,rep,name=items"`
}

type RuleResource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	Spec              RuleSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
}

type RuleSpec struct {
	Priority     int64          `json:"priority"`
	Type         string         `json:"type"`
	Domain       string         `json:"domain"`
	URL          string         `json:"url"`
	DSL          string         `json:"dsl"`
	Description  string         `json:"description"`
	ServiceGroup *ServicceGroup `json:"serviceGroup,omitempty"`
	Source       *SourceInfo    `json:"source,omitempty"`
}

type ServicceGroup struct {
	SessionAffinityPolicy    string    `json:"session_affinity_policy,omitempty"`
	SessionAffinityAttribute string    `json:"session_affinity_attribute,omitempty"`
	Services                 []Service `json:"services"`
}

type Service struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Port      int    `json:"port"`
	Weight    int    `json:"weight"`
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
