package portinfo

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Option func(configMap *v1.ConfigMap)

func defaultLabel(name string) Option {
	return func(cm *v1.ConfigMap) {
		if cm == nil {
			return
		}
		cm.Labels = map[string]string{
			"port-info": "true",
			"name":      name,
		}
	}
}

func AddLabel(labels map[string]string) Option {
	return func(cm *v1.ConfigMap) {
		if cm == nil {
			return
		}
		if cm.Labels == nil {
			cm.Labels = map[string]string{}
		}
		for k, v := range labels {
			cm.Labels[k] = v
		}
	}
}

func SetOwnerRefs(reference []metav1.OwnerReference) Option {
	return func(configMap *v1.ConfigMap) {
		if configMap == nil {
			return
		}
		configMap.OwnerReferences = reference
	}
}
