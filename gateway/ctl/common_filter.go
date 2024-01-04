package ctl

import (
	"context"
	"fmt"
	"sort"

	. "alauda.io/alb2/gateway"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gv1b1t "sigs.k8s.io/gateway-api/apis/v1beta1"
)

type CommonFiliter struct {
	log logr.Logger
	c   client.Client
	ctx context.Context
}

func (c *CommonFiliter) Name() string {
	return "CommonFiliter"
}

func (c *CommonFiliter) FilteListener(gateway client.ObjectKey, ls []*Listener, allls []*Listener) {
	c.filteListenerConflictPorotocol(gateway, ls, allls)
	c.filteListenerInvalidKind(gateway, ls, allls)
}

func (c *CommonFiliter) filteListenerInvalidKind(gateway client.ObjectKey, ls []*Listener, allls []*Listener) {
	for _, l := range ls {
		if l.AllowedRoutes == nil {
			continue
		}
		// invalidKindState:= In
		invalidskinds := []string{}
		hasinvalid := false
		for _, k := range l.AllowedRoutes.Kinds {
			if !SUPPORT_KIND_SET.Contains(string(k.Kind)) {
				hasinvalid = true
				invalidskinds = append(invalidskinds, string(k.Kind))
			}
		}
		if hasinvalid {
			allkindinvalid := len(invalidskinds) == len(l.AllowedRoutes.Kinds)
			l.status.invalidKind(allkindinvalid, invalidskinds)
		}
	}
}

func (c *CommonFiliter) filteListenerConflictPorotocol(gateway client.ObjectKey, ls []*Listener, allls []*Listener) {
	// TODO distinguish between protocol conflict and hostname conflict
	log := c.log.WithValues("gateway", gateway)
	// 1. mark listener conflict when port+protocol+hostname+port are the same
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

func (c *CommonFiliter) FilteRoute(ref gv1b1t.ParentReference, r *Route, ls *Listener) bool {
	// allow routes
	key := client.ObjectKeyFromObject(r.route.GetObject())
	accept, msg := c.routeCouldAttach(key, ls.gateway, &ls.Listener)
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

func (c *CommonFiliter) routeCouldAttach(route client.ObjectKey, gateway client.ObjectKey, ls *gv1b1t.Listener) (bool, string) {
	from := gv1b1t.NamespacesFromSame
	if ls.AllowedRoutes.Namespaces.From != nil {
		from = *ls.AllowedRoutes.Namespaces.From
	}
	if from == gv1b1t.NamespacesFromSame {
		if gateway.Namespace != route.Namespace {
			return false, fmt.Sprintf("gateway %v only allow route from same ns", gateway)
		}
	}

	if from == gv1b1t.NamespacesFromSelector && validNsSelector(ls) {
		ns := corev1.Namespace{}
		err := c.c.Get(c.ctx, client.ObjectKey{Name: route.Namespace}, &ns)
		if err != nil {
			return false, fmt.Sprintf("ns selector,get ns %s fail %v", route.Namespace, err)
		}
		nssel := ls.AllowedRoutes.Namespaces.Selector
		sel, err := metav1.LabelSelectorAsSelector(nssel)
		if err != nil {
			return false, fmt.Sprintf("ns selector, invalid selector: %v %v", nssel, err)
		}
		match := sel.Matches(labels.Set(ns.Labels))
		if !match {
			return false, "ns selector not match"
		}
	}
	return true, ""
}

func validNsSelector(ls *gv1b1t.Listener) bool {
	if ls == nil {
		return false
	}
	if ls.AllowedRoutes == nil {
		return false
	}
	if ls.AllowedRoutes.Namespaces.Selector == nil {
		return false
	}
	return true
}
