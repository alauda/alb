package gateway

import (
	gv1b1t "sigs.k8s.io/gateway-api/apis/v1beta1"
)

type Option func(service *gv1b1t.GatewayClass)

func AddLabel(labels map[string]string) Option {
	return func(ic *gv1b1t.GatewayClass) {
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
