package ctl

import (
	"fmt"

	. "alauda.io/alb2/gateway"
	gv1b1t "sigs.k8s.io/gateway-api/apis/v1beta1"
)

func generateSupportKind(protocol gv1b1t.ProtocolType, supportKindMap map[string][]string) []gv1b1t.RouteGroupKind {
	ret := []gv1b1t.RouteGroupKind{}
	group := gv1b1t.Group(GATEWAY_GROUP)
	for _, k := range supportKindMap[string(protocol)] {
		ret = append(ret, gv1b1t.RouteGroupKind{
			Group: &group,
			Kind:  gv1b1t.Kind(k),
		})
	}
	return ret
}

func UpdateRouteStatus(r CommonRoute, f func([]gv1b1t.RouteParentStatus) []gv1b1t.RouteParentStatus) ([]gv1b1t.RouteParentStatus, error) {
	switch route := r.(type) {
	case *HTTPRoute:
		route.Status.Parents = f(route.Status.Parents)
		return route.Status.Parents, nil
	case *TCPRoute:
		route.Status.Parents = f(route.Status.Parents)
		return route.Status.Parents, nil
	case *TLSRoute:
		route.Status.Parents = f(route.Status.Parents)
		return route.Status.Parents, nil
	case *UDPRoute:
		route.Status.Parents = f(route.Status.Parents)
		return route.Status.Parents, nil
	}
	return nil, fmt.Errorf("unsupported route type %T", r)
}
