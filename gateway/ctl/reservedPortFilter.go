package ctl

import (
	"fmt"

	"github.com/go-logr/logr"
	gv1b1t "sigs.k8s.io/gateway-api/apis/v1"
)

const RouteNotReadyReasonReservedPort = "ReservedPortUsed"

type ReservedPortFilter struct {
	log           logr.Logger
	reservedPorts map[int]bool
}

func NewReservedPortFilter(log logr.Logger, ports []int) ReservedPortFilter {
	m := map[int]bool{}
	for _, p := range ports {
		m[p] = true
	}
	log.Info("reservedPorts", "ports", m)
	return ReservedPortFilter{
		log:           log,
		reservedPorts: m,
	}
}

func (c *ReservedPortFilter) Name() string {
	return "ReservedPortFilter"
}

func (c *ReservedPortFilter) FilteRoute(ref gv1b1t.ParentReference, r *Route, ls *Listener) bool {
	if c.reservedPorts[int(ls.Port)] {
		r.unAllowRouteWithReason(ref, fmt.Sprintf("%v is our reserved port", c.reservedPorts), RouteNotReadyReasonReservedPort)
		return false
	}
	return true
}
