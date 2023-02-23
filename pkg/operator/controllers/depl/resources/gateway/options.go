package gateway

import (
	"sigs.k8s.io/gateway-api/apis/v1alpha2"
)

type Option func(service *v1alpha2.GatewayClass)

func AddLabel(labels map[string]string) Option {
	return func(ic *v1alpha2.GatewayClass) {
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

func defaultLabel(baseDomain, name string) Option {
	labels := map[string]string{
		"alb2." + baseDomain + "/gatewayclass": name,
	}
	return func(gc *v1alpha2.GatewayClass) {
		if gc == nil {
			return
		}
		gc.Labels = labels
	}
}
