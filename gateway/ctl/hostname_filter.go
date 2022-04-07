package ctl

import (
	"context"
	"strings"

	. "alauda.io/alb2/gateway"
	"github.com/go-logr/logr"
	"github.com/samber/lo"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayType "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

type HostNameFilter struct {
	log logr.Logger
	c   client.Client
	ctx context.Context
}

func (c *HostNameFilter) Name() string {
	return "HostNameFilter"
}

func (c *HostNameFilter) FilteRoute(ref gatewayType.ParentRef, r *Route, ls *Listener) bool {
	// allow routes
	lsHost := ls.Hostname
	if lsHost == nil {
		return true
	}

	h, ok := r.route.(*HTTPRoute)
	// only focus on http route.
	if !ok {
		return true
	}

	routeHost := lo.Map(h.Spec.Hostnames, func(s gatewayType.Hostname, _ int) string { return string(s) })

	domains := FindIntersection(string(*lsHost), routeHost)
	if len(domains) == 0 {
		r.unAllowRoute(ref, "no intersection hostname")
		return false
	}
	return true
}

func FindIntersection(lsHost string, routeHost []string) []string {
	ret := []string{}
	for _, rh := range routeHost {
		if matchDomain(lsHost, rh) {
			ret = append(ret, rh)
		}
	}
	return ret
}

func matchDomain(host, route string) bool {
	hostIsWildcard := strings.HasPrefix(host, "*.")
	routeIsWildcard := strings.HasPrefix(route, "*.")
	if hostIsWildcard && !routeIsWildcard {
		hostWithoutWildcard := strings.TrimPrefix(host, "*")
		return strings.HasSuffix(route, hostWithoutWildcard)
	}
	if !hostIsWildcard && routeIsWildcard {
		routeWithoutWildcard := strings.TrimPrefix(route, "*")
		return strings.HasSuffix(host, routeWithoutWildcard)
	}
	return host == route
}
