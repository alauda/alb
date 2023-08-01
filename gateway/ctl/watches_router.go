package ctl

import (
	"alauda.io/alb2/utils"
	"k8s.io/apimachinery/pkg/types"
	ctrlBuilder "sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	gv1a2t "sigs.k8s.io/gateway-api/apis/v1alpha2"
	gv1b1t "sigs.k8s.io/gateway-api/apis/v1beta1"
)

func (g *GatewayReconciler) watchRoutes(b *ctrlBuilder.Builder) *ctrlBuilder.Builder {
	log := g.log.WithName("watchroute")
	ctx := g.ctx
	c := g.c
	// TODO merge predicate and eventhandle
	predicateFunc := func(object client.Object, eventType string) bool {
		find, keys, err := findGatewayByRouteObject(ctx, c, object, g.cfg.GatewaySelector)
		log.Info("route to gateway", "event", eventType, "route", client.ObjectKeyFromObject(object), "gateway", keys, "find", find, "err", err)
		if err != nil {
			log.Info("failed to filter route object", "event", eventType, "err", err)
			return false
		}
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

	eventhandler := handler.EnqueueRequestsFromMapFunc(func(o client.Object) []reconcile.Request {
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
				NamespacedName: types.NamespacedName{Namespace: key.Namespace, Name: key.Name},
			})
		}
		return reqs
	})

	// TODO upgrade to controller-runtime 0.11.1 for better log https://github.com/kubernetes-sigs/controller-runtime/pull/1687
	httpRoute := gv1b1t.HTTPRoute{}
	utils.AddTypeInformationToObject(scheme, &httpRoute)
	tcpRoute := gv1a2t.TCPRoute{}
	utils.AddTypeInformationToObject(scheme, &tcpRoute)
	tlspRoute := gv1a2t.TLSRoute{}
	utils.AddTypeInformationToObject(scheme, &tlspRoute)
	udpRoute := gv1a2t.UDPRoute{}
	utils.AddTypeInformationToObject(scheme, &udpRoute)

	b = b.Watches(&source.Kind{Type: &httpRoute}, eventhandler, options)
	b = b.Watches(&source.Kind{Type: &tcpRoute}, eventhandler, options)
	b = b.Watches(&source.Kind{Type: &tlspRoute}, eventhandler, options)
	b = b.Watches(&source.Kind{Type: &udpRoute}, eventhandler, options)
	return b
}
