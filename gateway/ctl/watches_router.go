package ctl

import (
	"context"

	"alauda.io/alb2/utils"
	ctrlBuilder "sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gv1b1t "sigs.k8s.io/gateway-api/apis/v1"
	gv1a2t "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

//nolint:errcheck
func (g *GatewayReconciler) watchRoutes(b *ctrlBuilder.Builder) *ctrlBuilder.Builder {
	log := g.log.WithName("watchroute")
	ctx := g.ctx
	c := g.c
	// TODO merge predicate and eventhandle
	predicateFunc := func(object client.Object, eventType string) bool {
		find, keys, err := findGatewayByRouteObject(ctx, c, object, g.cfg.GatewaySelector)
		if err != nil {
			log.Info("failed to filter route object", "event", eventType, "err", err)
			return false
		}
		if !find {
			return false
		}
		log.Info("route to gateway", "event", eventType, "route", client.ObjectKeyFromObject(object), "gateway", keys, "ver", object.GetResourceVersion(), "err", err)
		return find
	}
	routePredicate := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return predicateFunc(e.Object, "create")
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return predicateFunc(e.ObjectNew, "update")
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return predicateFunc(e.Object, "delete")
		},
	}
	// ignore status change
	options := ctrlBuilder.WithPredicates(routePredicate, predicate.GenerationChangedPredicate{})

	eventhandler := handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, o client.Object) []reconcile.Request {
		find, keys, err := findGatewayByRouteObject(ctx, c, o, g.cfg.GatewaySelector)
		log.Info("find gateway by route", "find", find, "key", keys, "err", err)
		if !find {
			return []reconcile.Request{}
		}
		if err != nil {
			log.Info("find gateway by route fail", "error", err)
			return []reconcile.Request{}
		}
		reqs := []reconcile.Request{}
		for _, key := range keys {
			reqs = append(reqs, reconcile.Request{
				NamespacedName: client.ObjectKey{Namespace: key.Namespace, Name: key.Name},
			})
		}
		return reqs
	})

	// TODO upgrade to controller-runtime 0.11.1 for better log https://github.com/kubernetes-sigs/controller-runtime/pull/1687
	httpRoute := gv1b1t.HTTPRoute{}
	_ = utils.AddTypeInformationToObject(scheme, &httpRoute)
	tcpRoute := gv1a2t.TCPRoute{}
	_ = utils.AddTypeInformationToObject(scheme, &tcpRoute)
	tlspRoute := gv1a2t.TLSRoute{}
	_ = utils.AddTypeInformationToObject(scheme, &tlspRoute)
	udpRoute := gv1a2t.UDPRoute{}
	_ = utils.AddTypeInformationToObject(scheme, &udpRoute)

	b = b.Watches(&httpRoute, eventhandler, options)
	b = b.Watches(&tcpRoute, eventhandler, options)
	b = b.Watches(&tlspRoute, eventhandler, options)
	b = b.Watches(&udpRoute, eventhandler, options)
	return b
}
