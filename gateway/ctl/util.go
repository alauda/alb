package ctl

import (
	"fmt"

	. "alauda.io/alb2/gateway"
	gv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func generateSupportKind(protocol gv1.ProtocolType, supportKindMap map[string][]string) []gv1.RouteGroupKind {
	ret := []gv1.RouteGroupKind{}
	group := gv1.Group(GATEWAY_GROUP)
	for _, k := range supportKindMap[string(protocol)] {
		ret = append(ret, gv1.RouteGroupKind{
			Group: &group,
			Kind:  gv1.Kind(k),
		})
	}
	return ret
}

func UpdateRouteStatus(r CommonRoute, f func([]gv1.RouteParentStatus) []gv1.RouteParentStatus) ([]gv1.RouteParentStatus, error) {
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

func GetRouteStatus(r CommonRoute) ([]gv1.RouteParentStatus, error) {
	switch route := r.(type) {
	case *HTTPRoute:
		return route.Status.Parents, nil
	case *TCPRoute:
		return route.Status.Parents, nil
	case *TLSRoute:
		return route.Status.Parents, nil
	case *UDPRoute:
		return route.Status.Parents, nil
	}
	return nil, fmt.Errorf("unsupported route type %T", r)
}
