package ctl

import (
	"context"

	. "alauda.io/alb2/gateway"
	. "alauda.io/alb2/gateway/utils"
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
