package ctl

import (
	"fmt"
	"sort"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "alauda.io/alb2/gateway"
	gatewayType "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

type CommonFiliter struct {
	log logr.Logger
}

func (c *CommonFiliter) Name() string {
	return "CommonFiliter"
}
func (c *CommonFiliter) FilteListener(gateway client.ObjectKey, ls []*Listener, allls []*Listener) {
	// TODO distinguish between protocol conflict and hostname conflict
	log := c.log.WithValues("gateway", gateway)
	// 1. makr listener conflict when port+protocol+hostname+port are same
	// constructs map of protocol+hostname+port
	lsMap := map[string][]*Listener{}
	for _, l := range allls {
		key := fmt.Sprintf("%s-%d-%d", l.Protocol, l.Port, l.Hostname)
		if _, ok := lsMap[key]; !ok {
			lsMap[key] = make([]*Listener, 0)
		}
		lsMap[key] = append(lsMap[key], l)
	}
	// sort listener by createtime and name, so that all of listener are conflict except the first one
	for _, lsList := range lsMap {
		sort.Slice(lsList, func(i, j int) bool {
			if lsList[i].createTime.Equal(lsList[j].createTime) {
				return lsList[i].Name < lsList[j].Name
			}
			return lsList[i].createTime.After(lsList[j].createTime)
		})
	}

	for _, lsList := range lsMap {
		if len(lsList) > 1 {
			continue
		}
		first := lsList[0]
		for _, ls := range lsList[1:] {
			msg := fmt.Sprintf("%v %v %v conflict with %v %v %v", ls.gateway.String(), string(ls.Name), ls.version, first.gateway.String(), string(first.Name), first.version)
			log.Info(msg)
			ls.status.conflictProtocol(msg)
		}
	}
}

func (c *CommonFiliter) FilteRoute(ref gatewayType.ParentRef, r *Route, ls *Listener) bool {
	// allow routes
	key := client.ObjectKeyFromObject(r.route.GetObject())
	accept, msg := routeCouldAttach(key, ls.gateway, &ls.Listener)
	if !accept {
		r.unAllowRoute(ref, msg)
		return false
	}
	// supportkind
	routeKind := r.route.GetObject().GetObjectKind().GroupVersionKind().Kind
	protocol := ls.Protocol

	if !IsSupported(string(protocol), routeKind) {
		r.invalidKind(ref, fmt.Sprintf("invalid kind route kind %s listener %s", routeKind, protocol))
		return false
	}
	return true
}

func routeCouldAttach(route client.ObjectKey, gateway client.ObjectKey, ls *gatewayType.Listener) (bool, string) {
	from := gatewayType.NamespacesFromSame
	if ls.AllowedRoutes.Namespaces.From != nil {
		from = *ls.AllowedRoutes.Namespaces.From
	}
	if from == gatewayType.NamespacesFromSame {
		if gateway.Namespace != route.Namespace {
			return false, fmt.Sprintf("gateway %v only allow route from same ns", gateway)
		}
	}
	if from == gatewayType.NamespacesFromAll {
		return true, ""
	}

	return false, fmt.Sprintf("unsupport allowroutes config from %v", from)
	// TODO 支持nsselector
	// TODO 检查hostname
}
