package ingressclass

import (
	"alauda.io/alb2/pkg/operator/toolkit"
	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Template struct {
	name         string
	baseDomain   string
	defaultClass bool
}

func NewTemplate(namespace, name, baseDomain string, defaultClass bool) *Template {
	return &Template{
		name:         name,
		baseDomain:   baseDomain,
		defaultClass: defaultClass,
	}
}

func (t *Template) Generate(options ...Option) *netv1.IngressClass {
	defaultClassStr := "false"
	if t.defaultClass {
		defaultClassStr = "true"
	}
	annotation := map[string]string{
		"ingressclass.kubernetes.io/is-default-class": defaultClassStr,
	}
	ic := &netv1.IngressClass{
		ObjectMeta: metav1.ObjectMeta{
			Name:        t.name,
			Annotations: annotation,
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
