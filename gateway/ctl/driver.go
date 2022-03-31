package ctl

import (
	"context"
	"fmt"

	. "alauda.io/alb2/gateway"
	. "alauda.io/alb2/utils/log"
	corev1 "k8s.io/api/core/v1"
	errors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayType "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

func ListListenerByClass(ctx context.Context, c client.Client, name string) ([]*Listener, error) {
	gateways := gatewayType.GatewayList{}
	if err := c.List(ctx, &gateways, &client.ListOptions{}); err != nil {
		return nil, err
	}
	gs := make([]gatewayType.Gateway, 0)
	for _, g := range gateways.Items {
		if string(g.Spec.GatewayClassName) == name {
			gs = append(gs, g)
		}
	}
	ret := []*Listener{}
	for _, g := range gs {
		for _, ls := range g.Spec.Listeners {
			ret = append(ret, &Listener{Listener: ls, gateway: client.ObjectKeyFromObject(&g)})
		}
	}
	return ret, nil
}

func ListRoutesByGateway(ctx context.Context, c client.Client, gateway client.ObjectKey) ([]*Route, error) {
	log := L().WithName(ALB_GATEWAY_CONTROLLER).WithValues("gateway", gateway.String())
	// TODO !!!! use client.object instead of xxroute
	httpRouteList := &gatewayType.HTTPRouteList{}
	tcpRouteList := &gatewayType.TCPRouteList{}
	tlsRouteList := &gatewayType.TLSRouteList{}
	udpRouteList := &gatewayType.UDPRouteList{}
	err := c.List(ctx, httpRouteList, &client.ListOptions{})
	if err != nil {
		return nil, err
	}
	err = c.List(ctx, tcpRouteList, &client.ListOptions{})
	if err != nil {
		return nil, err
	}
	err = c.List(ctx, tlsRouteList, &client.ListOptions{})
	if err != nil {
		return nil, err
	}
	err = c.List(ctx, udpRouteList, &client.ListOptions{})
	if err != nil {
		return nil, err
	}
	routes := []*Route{}
	httpCommonRoutes := []*Route{}
	tcpCommonRoutes := []*Route{}
	tlsCommonRoutes := []*Route{}
	udpCommonRoutes := []*Route{}

	for _, route := range httpRouteList.Items {
		ref := IsRefsToGateway(route.Spec.ParentRefs, gateway)
		log.V(4).Info("http route ref to gateway?", "result", ref, "route", client.ObjectKeyFromObject(&route))
		if ref {
			r := HTTPRoute(route)
			httpCommonRoutes = append(httpCommonRoutes, &Route{route: &r})
		}
	}
	log.Info("list http route", "total-len", len(httpRouteList.Items), "len", len(httpCommonRoutes))

	for _, route := range tlsRouteList.Items {
		ref := IsRefsToGateway(route.Spec.ParentRefs, gateway)
		log.V(4).Info("tls route ref to gateway?", "result", ref, "route", client.ObjectKeyFromObject(&route))
		if ref {
			r := TLSRoute(route)
			httpCommonRoutes = append(tlsCommonRoutes, &Route{route: &r})
		}
	}
	log.Info("list tls route", "total-len", len(tlsRouteList.Items), "len", len(tlsCommonRoutes))

	for _, route := range tcpRouteList.Items {
		ref := IsRefsToGateway(route.Spec.ParentRefs, gateway)
		log.V(4).Info("tls route ref to gateway?", "result", ref, "route", client.ObjectKeyFromObject(&route))
		if ref {
			r := TCPRoute(route)
			tcpCommonRoutes = append(tcpCommonRoutes, &Route{route: &r})
		}
	}
	log.Info("list tcp route", "total-len", len(tcpRouteList.Items), "len", len(tcpCommonRoutes))

	for _, route := range udpRouteList.Items {
		if IsRefsToGateway(route.Spec.ParentRefs, gateway) {
			r := UDPRoute(route)
			udpCommonRoutes = append(udpCommonRoutes, &Route{route: &r})
		}
	}
	log.Info("list udp route", "total-len", len(udpRouteList.Items), "len", len(udpCommonRoutes))

	routes = append(routes, httpCommonRoutes...)
	routes = append(routes, tlsCommonRoutes...)
	routes = append(routes, tcpCommonRoutes...)
	routes = append(routes, udpCommonRoutes...)
	for _, r := range routes {
		r.status = make(map[string]RouteStatus)
	}
	return routes, nil
}

func findGatewayByRouteObject(ctx context.Context, c client.Client, object client.Object, class string) (bool, *client.ObjectKey, error) {
	log := L().WithName(ALB_GATEWAY_CONTROLLER).WithName("findgatewaybyroute").WithValues("route", client.ObjectKeyFromObject(object))
	// TODO a better way
	var refs []gatewayType.ParentRef
	switch object := object.(type) {
	case *gatewayType.HTTPRoute:
		refs = object.Spec.ParentRefs
	case *gatewayType.TCPRoute:
		refs = object.Spec.ParentRefs
	case *gatewayType.TLSRoute:
		refs = object.Spec.ParentRefs
	case *gatewayType.UDPRoute:
		refs = object.Spec.ParentRefs
	default:
		return false, nil, fmt.Errorf("invalid route type %v", client.ObjectKeyFromObject(object))
	}

	for _, ref := range refs {
		log.V(2).Info("check refs", "ref", RefsToString(ref))
		if ref.Kind != nil && *ref.Kind != GatewayKind {
			continue
		}
		if ref.Namespace == nil {
			log.Info("invalid ref namespace ignore", "ref", RefsToString(ref), "ignore", "true")
			continue
		}
		key := client.ObjectKey{Namespace: string(*ref.Namespace), Name: string(ref.Name)}
		gateway := gatewayType.Gateway{}
		err := c.Get(ctx, key, &gateway)

		if errors.IsNotFound(err) {
			log.Info("could not find gateway ref in route", "ref", RefsToString(ref), "ignore", "true")
			continue
		}
		if err != nil {
			log.Info("find gateway ref in route err", "ref", RefsToString(ref), "ignore", "true", "err", err)
			continue
		}
		if string(gateway.Spec.GatewayClassName) != class {
			log.V(2).Info("find gateway ref to other controler", "gateway", key, "class", class)
		}
		return true, &key, nil
	}
	return false, nil, nil
}

func getAlbPodIp(ctx context.Context, c client.Client, ns string, domain, name string) ([]string, error) {
	endpoints := corev1.Endpoints{}
	err := c.Get(ctx, client.ObjectKey{Namespace: ns, Name: name}, &endpoints)
	if err != nil {
		return nil, err
	}
	ret := []string{}
	for _, ss := range endpoints.Subsets {
		for _, a := range ss.Addresses {
			ret = append(ret, a.IP)
		}
	}
	return ret, nil
}
