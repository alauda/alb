package ingressclass

import netv1 "k8s.io/api/networking/v1"

type Option func(service *netv1.IngressClass)

func AddLabel(labels map[string]string) Option {
	return func(ic *netv1.IngressClass) {
		if ic == nil {
			return
		}
		if ic.Labels == nil {
			ic.Labels = map[string]string{}
		}
		for k, v := range labels {
			ic.Labels[k] = v
		}
	}
}
