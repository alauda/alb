package ctl

import (
	"context"
	"reflect"

	"alauda.io/alb2/utils"
	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/types"
	ctrlBuilder "sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	gv1b1t "sigs.k8s.io/gateway-api/apis/v1"

	albv2 "alauda.io/alb2/pkg/apis/alauda/v2beta1"
)

func (g *GatewayReconciler) watchAlb(b *ctrlBuilder.Builder) *ctrlBuilder.Builder {
	log := g.log.WithName("watchalb")
	c := g.c
	IsMe := func(a client.Object) bool {
		alb, ok := a.(*albv2.ALB2)
		if !ok {
			return false
		}
		return alb.Name == g.albcfg.Name && alb.Namespace == g.albcfg.Ns
	}

	predicate := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return IsMe(e.Object)
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			old, ok := e.ObjectOld.(*albv2.ALB2)
			if !ok {
				return false
			}
			me := IsMe(e.ObjectOld)
			if !me {
				return false
			}
			latest, ok := e.ObjectNew.(*albv2.ALB2)
			if !ok {
				return false
			}
			oldAddress := pickAlbAddress(old)
			newAddress := pickAlbAddress(latest)
			addressChange := !reflect.DeepEqual(oldAddress, newAddress)
			if addressChange {
				log.Info("alb address change", "old", oldAddress, "new", newAddress, "name", old.Name, "ns", old.Namespace)
			}
			statusChange := old.Status.State != latest.Status.State
			if statusChange {
				log.Info("alb status change", "diff", cmp.Diff(old.Status, latest.Status))
			}
			return addressChange || statusChange
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return false
		},
	}
	options := ctrlBuilder.WithPredicates(predicate)

	eventhandler := handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, o client.Object) (ret []reconcile.Request) {
		if g.cfg.GatewaySelector.GatewayClass != nil {
			list := &gv1b1t.GatewayList{}
			err := c.List(ctx, list)
			if err != nil {
				log.Error(err, "gatewayclass mode alb change. but list gateway fail")
				return
			}
			keys := []reconcile.Request{}
			for _, gw := range list.Items {
				if string(gw.Spec.GatewayClassName) == *g.cfg.GatewaySelector.GatewayClass {
					log.Info("gatewayclass mode alb change reconcile gateay", "gw-name", gw.Name, "gw-ns", gw.Namespace)
					keys = append(keys, reconcile.Request{
						NamespacedName: types.NamespacedName{
							Namespace: gw.Namespace,
							Name:      gw.Name,
						},
					})
				}
			}
			return keys
		}
		if g.cfg.GatewaySelector.GatewayName != nil {
			log.Info("gateways mode alb change reconcile gateay")
			return []reconcile.Request{
				{
					NamespacedName: *g.cfg.GatewaySelector.GatewayName,
				},
			}
		}
		return
	})

	alb := albv2.ALB2{}
	err := utils.AddTypeInformationToObject(scheme, &alb)
	if err != nil {
		log.Error(err, "failed to add type information to object")
	}
	b = b.Watches(&alb, eventhandler, options)
	return b
}
