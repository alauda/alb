package portinfo

import (
	"alauda.io/alb2/pkg/operator/toolkit"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Template struct {
	namespace string
	name      string
	data      string
}

func NewTemplate(namespace string, name string, data string) *Template {
	return &Template{
		namespace: namespace,
		name:      name,
		data:      data,
	}
}

func (t *Template) Generate(options ...Option) *v1.ConfigMap {
	name := toolkit.FmtKeyBySep("-", t.name, "port-info")
	cm := &v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: t.namespace,
		},
		Data: map[string]string{
			"range": t.data,
		},
	}
	defaultOptions := []Option{
		defaultLabel(t.name),
	}
	for _, op := range defaultOptions {
		op(cm)
	}
	for _, op := range options {
		op(cm)
	}
	return cm
}
