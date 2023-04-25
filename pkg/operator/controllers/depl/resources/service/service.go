package service

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type Template struct {
	namespace string
	name      string
	protocol  v1.Protocol
	metrics   int32
}

func NewTemplate(namespace, name string, protocol v1.Protocol, metircs int32) *Template {
	return &Template{
		namespace: namespace,
		name:      name,
		protocol:  protocol,
		metrics:   metircs,
	}
}

func (t *Template) Generate(options ...Option) *v1.Service {
	svc := &v1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      t.name,
			Namespace: t.namespace,
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{
				{
					Name:     "metrics",
					Protocol: t.protocol,
					Port:     t.metrics,
					TargetPort: intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: t.metrics,
					},
				},
			},
			Type:            "ClusterIP",
			SessionAffinity: "None",
		},
	}
	defaultOptions := []Option{
		defaultSelector(t.name),
	}
	for _, op := range defaultOptions {
		op(svc)
	}
	for _, op := range options {
		op(svc)
	}
	return svc
}
