package framework

import (
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	Logf("c %+v %v", g.Status.Conditions, g.Generation)
	if len(g.Status.Conditions) != 1 {
		Logf("len condition not 1 ")
		return false
	}
	c := g.Status.Conditions[0]
	return c.Type == "Ready" && c.Status == "True" && c.ObservedGeneration == g.Generation
}

func (g Gateway) SameAddress(ips []string) bool {
	Logf("status address %s %+v", g.Name, g.Status.Addresses)
	if len(ips) != len(g.Status.Addresses) {
		Logf("ip not same %v %v", len(ips), len(g.Status.Addresses))
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
			Logf("could not found -%s-", ip)
			return false
		}
	}
	return true
}

func (g Gateway) LsAttachedRoutes() map[string]int32 {
	Logf("c %+v", g.Status.Conditions)
	ls_routes := make(map[string]int32)
	for _, listener := range g.Status.Listeners {
		ls_routes[string(listener.Name)] = listener.AttachedRoutes
	}
	return ls_routes
}

func (f *Framework) CheckGatewayStatus(key client.ObjectKey, ip []string) (bool, error) {
	g, err := f.GetGatewayClient().GatewayV1alpha2().Gateways(key.Namespace).Get(f.ctx, key.Name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !Gateway(*g).SameAddress(ip) {
		return false, nil
	}
	if !Gateway(*g).Ready() {
		return false, nil
	}
	return true, nil
}

func NewParentRef(ns, name, section string) gatewayType.ParentRef {
	gName := gatewayType.ObjectName(name)
	gNs := gatewayType.Namespace(ns)
	gSection := gatewayType.SectionName(section)
	return gatewayType.ParentRef{
		Namespace:   &gNs,
		Name:        gName,
		SectionName: &gSection,
	}
}

func (f *Framework) WaitHttpRouteStatus(ns, name string, ref gatewayType.ParentRef, check func(status gatewayType.RouteParentStatus) (bool, error)) {
	f.Wait(func() (bool, error) {
		route, err := f.GetGatewayClient().GatewayV1alpha2().HTTPRoutes(ns).Get(f.ctx, name, metav1.GetOptions{})
		if err != nil {
			Logf("err %v", err)
			return false, nil
		}
		found := false
		for _, s := range route.Status.Parents {
			pref := s.ParentRef
			refEq := pref.Name == ref.Name && *pref.Namespace == *ref.Namespace && *pref.SectionName == *ref.SectionName
			if refEq {
				found = true
				ret, err := check(s)
				Logf("wait http route status fail ret %v err %v", ret, err)
				if ret && err == nil {
					return true, nil
				}
			}
		}
		if !found {
			Logf("wait http route status could not found route status %+v", route.Status)
		}
		return false, nil
	})
}
