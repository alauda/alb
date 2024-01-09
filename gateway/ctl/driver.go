package ctl

import (
	"context"
	"fmt"

	"alauda.io/alb2/config"
	. "alauda.io/alb2/gateway"
	. "alauda.io/alb2/utils/log"

	alb2v2 "alauda.io/alb2/pkg/apis/alauda/v2beta1"
	errors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gv1 "sigs.k8s.io/gateway-api/apis/v1"
	gv1a2t "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

func ListListener(ctx context.Context, c client.Client, sel config.GatewaySelector) ([]*Listener, error) {
	gs := getGatewayList(ctx, c, sel)
	ret := []*Listener{}
	for _, g := range gs {
		for _, ls := range g.Spec.Listeners {
			ret = append(ret, &Listener{Listener: ls, gateway: client.ObjectKeyFromObject(&g)})
		}
	}
	return ret, nil
}

func getGatewayList(ctx context.Context, c client.Client, sel config.GatewaySelector) []gv1.Gateway {
	gs := make([]gv1.Gateway, 0)
	if sel.GatewayClass != nil {
		class := *sel.GatewayClass
		gateways := gv1.GatewayList{}
		if err := c.List(ctx, &gateways, &client.ListOptions{}); err != nil {
			return nil
		}
		for _, g := range gateways.Items {
			if string(g.Spec.GatewayClassName) == class {
				gs = append(gs, g)
			}
		}
	}
	if sel.GatewayName != nil {
		gateway := gv1.Gateway{}
		err := c.Get(ctx, *sel.GatewayName, &gateway)
		if err != nil {
			return nil
		}
		gs = []gv1.Gateway{gateway}
	}
	return gs
}

func ListRoutesByGateway(ctx context.Context, c client.Client, gateway client.ObjectKey) ([]*Route, error) {
	log := L().WithName(ALB_GATEWAY_CONTROLLER).WithValues("gateway", gateway.String())
	// TODO !!!! use client.object instead of xxroute
	httpRouteList := &gv1.HTTPRouteList{}
	tcpRouteList := &gv1a2t.TCPRouteList{}
	tlsRouteList := &gv1a2t.TLSRouteList{}
	udpRouteList := &gv1a2t.UDPRouteList{}
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
			tlsCommonRoutes = append(tlsCommonRoutes, &Route{route: &r})
		}
	}
	log.Info("list tls route", "total-len", len(tlsRouteList.Items), "len", len(tlsCommonRoutes))

	for _, route := range tcpRouteList.Items {
		ref := IsRefsToGateway(route.Spec.ParentRefs, gateway)
		log.V(4).Info("tcp route ref to gateway?", "result", ref, "route", client.ObjectKeyFromObject(&route))
		if ref {
			r := TCPRoute(route)
			tcpCommonRoutes = append(tcpCommonRoutes, &Route{route: &r})
		}
	}
	log.Info("list tcp route", "total-len", len(tcpRouteList.Items), "len", len(tcpCommonRoutes))

	for _, route := range udpRouteList.Items {
		ref := IsRefsToGateway(route.Spec.ParentRefs, gateway)
		log.V(4).Info("udp route ref to gateway?", "result", ref, "route", client.ObjectKeyFromObject(&route))
		if ref {
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

func findGatewayByRouteObject(ctx context.Context, c client.Client, object client.Object, sel config.GatewaySelector) (bool, []client.ObjectKey, error) {
	log := L().WithName(ALB_GATEWAY_CONTROLLER).WithName("findgatewaybyroute").WithValues("route", client.ObjectKeyFromObject(object))
	// TODO a better way
	var refs []gv1.ParentReference
	switch object := object.(type) {
	case *gv1.HTTPRoute:
		refs = object.Spec.ParentRefs
	case *gv1a2t.TCPRoute:
		refs = object.Spec.ParentRefs
	case *gv1a2t.TLSRoute:
		refs = object.Spec.ParentRefs
	case *gv1a2t.UDPRoute:
		refs = object.Spec.ParentRefs
	default:
		return false, nil, fmt.Errorf("invalid route type %v", client.ObjectKeyFromObject(object))
	}
	keys := map[string]client.ObjectKey{}
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
		if sel.GatewayName != nil {
			if key == *sel.GatewayName {
				keys[key.String()] = key
			}
		}
		if sel.GatewayClass != nil {
			class := *sel.GatewayClass
			gateway := gv1.Gateway{}
			err := c.Get(ctx, key, &gateway)

			if errors.IsNotFound(err) {
				log.Info("could not find gateway ref in route", "ref", RefsToString(ref), "ignore", "true")
				continue
			}
			if err != nil {
				log.Info("find gateway ref in route err", "ref", RefsToString(ref), "ignore", "true", "err", err)
				continue
			}
			if string(gateway.Spec.GatewayClassName) == class {
				log.V(5).Info("same class", "ref", RefsToString(ref), "class", class)
				keys[key.String()] = key
			}
		}
	}
	keyList := []client.ObjectKey{}
	for _, key := range keys {
		keyList = append(keyList, key)
	}
	log.V(5).Info("find gateway", "len", len(keyList))
	return len(keyList) != 0, keyList, nil
}

func getAlb(ctx context.Context, c client.Client, ns string, name string) (*alb2v2.ALB2, error) {
	alb := &alb2v2.ALB2{}
	err := c.Get(ctx, client.ObjectKey{Namespace: ns, Name: name}, alb)
	if err != nil {
		return nil, err
	}
	return alb, nil
}

func pickAlbAddress(alb *alb2v2.ALB2) []string {
	return alb.GetAllAddress()
}
