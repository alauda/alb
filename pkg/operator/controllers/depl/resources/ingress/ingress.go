package ingress

import (
	"alauda.io/alb2/pkg/operator/toolkit"
	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Template struct {
	namespace  string
	name       string
	baseDomain string
}

func NewTemplate(namespace, name, baseDomain string) *Template {
	return &Template{
		namespace:  namespace,
		name:       name,
		baseDomain: baseDomain,
	}
}

func (t *Template) Generate(options ...Option) *netv1.IngressClass {
	ic := &netv1.IngressClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: t.name,
		},
		Spec: netv1.IngressClassSpec{
			Controller: toolkit.FmtKeyBySep("/", t.baseDomain, "alb2"),
		},
	}
	for _, op := range options {
		op(ic)
	}
	return ic
}
