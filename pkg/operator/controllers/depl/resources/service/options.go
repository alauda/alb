package service

import (
	"alauda.io/alb2/pkg/operator/toolkit"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Option func(service *v1.Service)

func SetOwnerRefs(reference []metav1.OwnerReference) Option {
	return func(service *v1.Service) {
		if service == nil {
			return
		}
		service.OwnerReferences = reference
	}
}

func SetServiceType(t v1.ServiceType) Option {
	return func(service *v1.Service) {
		if service == nil {
			return
		}
		service.Spec.Type = t
	}
}

func AddMetalLBAnnotation(namespace, name string) Option {
	return func(service *v1.Service) {
		if service == nil {
			return
		}
		if service.Annotations == nil {
			service.Annotations = map[string]string{}
		}
		service.Annotations["metallb.universe.tf/allow-shared-ip"] = toolkit.FmtKeyBySep("/", namespace, name)
	}
}

func AddLabel(labels map[string]string) Option {
	return func(service *v1.Service) {
		if service == nil {
			return
		}
		if service.Labels == nil {
			service.Labels = map[string]string{}
		}
		for k, v := range labels {
			service.Labels[k] = v
		}
	}
}

func defaultSelector(name string) Option {
	return func(service *v1.Service) {
		if service == nil {
			return
		}
		labels := map[string]string{
			"service_name": "alb2-" + name,
		}
		service.Spec.Selector = labels
	}
}
