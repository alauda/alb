package framework

import (
	gatewayType "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

type Gateway gatewayType.Gateway

func (g Gateway) WaittingController() bool {
	if len(g.Status.Conditions) != 1 {
		return false
	}
	msg := g.Status.Conditions[0].Message
	Logf("c %s", msg)
	return msg == "Waiting for controller"
}

func (g Gateway) Ready() bool {
	Logf("c %+v", g.Status.Conditions)
	if len(g.Status.Conditions) != 1 {
		return false
	}
	c := g.Status.Conditions[0]
	return c.Type == "Ready" && c.Status == "True" && c.ObservedGeneration == g.Generation
}

func (g Gateway) SameAddress(ips []string) bool {
	Logf("status address %+v", g.Status.Addresses)
	if len(ips) != len(g.Status.Addresses) {
		return false
	}
	ipMap := map[string]bool{}
	for _, a := range g.Status.Addresses {
		if a.Type != nil && *a.Type != gatewayType.IPAddressType {
			Logf("invalid address type %v", *a.Type)
			return false
		}
		ipMap[a.Value] = true
	}
	for _, ip := range ips {
		if !ipMap[ip] {
			Logf("could not found %s", ip)
			return false
		}
	}
	return true
}

func (g Gateway) Ls_attached_routes() map[string]int32 {
	Logf("c %+v", g.Status.Conditions)
	ls_routes := make(map[string]int32)
	for _, listener := range g.Status.Listeners {
		ls_routes[string(listener.Name)] = listener.AttachedRoutes
	}
	return ls_routes
}
