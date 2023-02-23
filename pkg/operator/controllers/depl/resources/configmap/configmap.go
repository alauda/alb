package configmap

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Template struct {
	namespace string
	name      string
}

func NewTemplate(namespace string, name string) *Template {
	return &Template{
		namespace: namespace,
		name:      name,
	}
}

func (t *Template) Generate(options ...Option) *v1.ConfigMap {
	cm := &v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      t.name,
			Namespace: t.namespace,
		},
		Data: map[string]string{
			"http":          HTTP,
			"http_server":   HTTPSERVER,
			"grpc_server":   GRPCSERVER,
			"stream-common": STREAM_COMMON,
			"stream-tcp":    STREAM_TCP,
			"stream-udp":    STREAM_UDP,
			"upstream":      UPSTREAM,
		},
	}
	for _, op := range options {
		op(cm)
	}
	return cm
}
