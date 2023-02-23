package configmap

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Option func(configMap *v1.ConfigMap)

func WithBindNIC(nic string) Option {
	return func(configMap *v1.ConfigMap) {
		if configMap == nil {
			return
		}
		if configMap.Data == nil {
			configMap.Data = map[string]string{}
		}
		configMap.Data["bind_nic"] = nic
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
