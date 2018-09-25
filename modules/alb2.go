package modules

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ProtoHTTP  = "http"
	ProtoHTTPS = "https"
	ProtoTCP   = "tcp"
	ProtoUDP   = "udp"
)

type Alb2Resource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	Spec              Alb2Spec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
}

type Alb2Spec struct {
	Address        string `json:"address"`
	BindAddress    string `json:"bind_address"`
	LoadBalancerID string `json:"iaas_id"`
	Type           string `json:"type"`
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
