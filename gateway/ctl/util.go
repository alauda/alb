package ctl

import (
	"fmt"

	. "alauda.io/alb2/gateway"
	gatewayType "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

func generateSupportKind(protocol gatewayType.ProtocolType, supportKindMap map[string][]string) []gatewayType.RouteGroupKind {
	ret := []gatewayType.RouteGroupKind{}
	group := gatewayType.Group(GATEWAY_GROUP)
	for _, k := range supportKindMap[string(protocol)] {
		ret = append(ret, gatewayType.RouteGroupKind{
			Group: &group,
			Kind:  gatewayType.Kind(k),
		})
	}
	return ret
}

func UpdateRouteStatus(r CommonRoute, f func([]gatewayType.RouteParentStatus) []gatewayType.RouteParentStatus) ([]gatewayType.RouteParentStatus, error) {
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
