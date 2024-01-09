package framework

import (
	"context"
	"fmt"

	. "alauda.io/alb2/utils/test_utils"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gv1 "sigs.k8s.io/gateway-api/apis/v1"
)

type Gateway gv1.Gateway

func (g Gateway) WaittingController() bool {
	Logf("wait controller %+v", PrettyJson(g.Status))
	msg := g.Status.Conditions[0].Message
	Logf("wait controller %s", msg)
	return msg == "Waiting for controller"
}

func (g Gateway) Ready() bool {
	Logf("ready %+v %v", g.Status.Conditions, g.Generation)
	for _, c := range g.Status.Conditions {
		if c.Type == "Ready" && c.Status == "True" && c.ObservedGeneration == g.Generation {
			return true
		}
	}
	return false
}

func (g Gateway) SameAddress(ips []string) bool {
	Logf("status address %s %+v", g.Name, g.Status.Addresses)
	if len(ips) != len(g.Status.Addresses) {
		Logf("ip not same %v %v", len(ips), len(g.Status.Addresses))
		return false
	}
	ipMap := map[string]bool{}
	for _, a := range g.Status.Addresses {
		if a.Type != nil && *a.Type != gv1.IPAddressType {
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

type GatewayAssert struct {
	Cli *K8sClient
	Ctx context.Context
}

func NewGatewayAssert(cli *K8sClient, ctx context.Context) *GatewayAssert {
	return &GatewayAssert{
		Cli: cli,
		Ctx: ctx,
	}
}

func (ga *GatewayAssert) CheckGatewayStatus(key client.ObjectKey, ip []string) (bool, error) {
	g, err := ga.Cli.GetGatewayClient().GatewayV1().Gateways(key.Namespace).Get(ga.Ctx, key.Name, metav1.GetOptions{})
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

func NewParentRef(ns, name, section string) gv1.ParentReference {
	gName := gv1.ObjectName(name)
	gNs := gv1.Namespace(ns)
	gSection := gv1.SectionName(section)
	return gv1.ParentReference{
		Namespace:   &gNs,
		Name:        gName,
		SectionName: &gSection,
	}
}

func (ga *GatewayAssert) WaitHttpRouteStatus(ns, name string, ref gv1.ParentReference, check func(status gv1.RouteParentStatus) (bool, error)) {
	ga.WaitRouteStatus("http", ns, name, ref, check)
}

func (ga *GatewayAssert) WaitTcpRouteStatus(ns, name string, ref gv1.ParentReference, check func(status gv1.RouteParentStatus) (bool, error)) {
	ga.WaitRouteStatus("tcp", ns, name, ref, check)
}

func (ga *GatewayAssert) WaitRouteStatus(kind string, ns, name string, ref gv1.ParentReference, check func(status gv1.RouteParentStatus) (bool, error)) {
	Wait(func() (bool, error) {
		status, err := ga.GetRouterStatus(ns, name, kind)
		if err != nil {
			Logf("err %v", err)
			return false, nil
		}
		found := false
		for _, s := range status.Parents {
			pref := s.ParentRef
			refEq := pref.Name == ref.Name && *pref.Namespace == *ref.Namespace && *pref.SectionName == *ref.SectionName
			if refEq {
				found = true
				ret, err := check(s)
				Logf("wait %s route status fail ret %v err %v", kind, ret, err)
				if ret && err == nil {
					return true, nil
				}
			}
		}
		if !found {
			Logf("wait %s route status could not found route status %+v", kind, status)
		}
		return false, nil
	})
}

func (ga *GatewayAssert) GetRouterStatus(ns string, name string, kind string) (*gv1.RouteStatus, error) {
	if kind == "http" {
		route, err := ga.Cli.GetGatewayClient().GatewayV1().HTTPRoutes(ns).Get(ga.Ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return &route.Status.RouteStatus, nil
	}
	if kind == "tcp" {
		route, err := ga.Cli.GetGatewayClient().GatewayV1alpha2().TCPRoutes(ns).Get(ga.Ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return &route.Status.RouteStatus, nil
	}
	return nil, fmt.Errorf("unsupported route %v", kind)
}

func (ga *GatewayAssert) WaitGateway(ns, name string, check func(g Gateway) (bool, error)) {
	Wait(func() (bool, error) {
		return TestEq(func() bool {
			g, err := ga.Cli.GetGatewayClient().GatewayV1().Gateways(ns).Get(ga.Ctx, name, metav1.GetOptions{})
			GinkgoAssert(err, "get gateway fail")
			ret, err := check(Gateway(*g))
			GinkgoAssert(err, "check fail")
			return ret
		}, "wait gateway"), nil
	})
}
