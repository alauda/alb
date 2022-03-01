package ctl

import (
	"context"
	"fmt"

	"alauda.io/alb2/config"
	. "alauda.io/alb2/gateway"
	"alauda.io/alb2/utils"
	. "alauda.io/alb2/utils/log"
	"github.com/go-logr/logr"
	errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlBuilder "sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayType "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

type GatewayReconciler struct {
	ctx                   context.Context
	class                 string
	controllerName        string
	c                     client.Client
	log                   logr.Logger
	invalidListenerfilter []ListenerFilter
	invalidRoutefilter    []RouteFilter
	supportKind           map[string][]string
}

type ListenerFilter interface {
	FilteListener(gateway client.ObjectKey, ls []*Listener, allls []*Listener)
	Name() string
}

type RouteFilter interface {
	FilteRoute(ref gatewayType.ParentRef, route *Route, ls *Listener) bool
	Name() string
}

func NewGatewayReconciler(ctx context.Context, c client.Client, log logr.Logger) GatewayReconciler {

	filter := CommonFiliter{log: log}
	listenerFilter := []ListenerFilter{
		&filter,
	}
	routeFilter := []RouteFilter{
		&filter,
	}

	return GatewayReconciler{
		c:                     c,
		log:                   log,
		ctx:                   ctx,
		class:                 GetClassName(), // we could only reconcile gateway which className is our.
		controllerName:        GetControllerName(),
		invalidListenerfilter: listenerFilter,
		invalidRoutefilter:    routeFilter,
	}
}

func (g *GatewayReconciler) SetupWithManager(mgr ctrl.Manager) error {
	b := ctrl.NewControllerManagedBy(mgr).
		For(&gatewayType.Gateway{}).
		WithEventFilter(predicate.GenerationChangedPredicate{})

	b = g.watchRoutes(b)
	// defualt rate limit should enough for use.
	b = b.WithOptions(controller.Options{RateLimiter: workqueue.DefaultControllerRateLimiter()})

	return b.Complete(g)
}

func (g *GatewayReconciler) watchRoutes(b *ctrlBuilder.Builder) *ctrlBuilder.Builder {
	log := g.log
	ctx := g.ctx
	c := g.c
	class := g.class
	predicateFunc := func(object client.Object, eventType string) bool {
		find, gateway, err := findGatewayByRouteObject(ctx, c, object, class)
		log.Info("route to gateway", "event", eventType, "route", client.ObjectKeyFromObject(object), "gateway", gateway, "find", find, "err", err)
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
		log := L().WithName(ALB_GATEWAY_CONTROLLER).WithValues("route", client.ObjectKeyFromObject(o))
		find, key, err := findGatewayByRouteObject(ctx, c, o, class)
		log.Info("find gateway by route", "find", find, "key", key, "err", err)
		if !find {
			return []reconcile.Request{}
		}
		if err != nil {
			log.Info("find gateway by route fail", "error", err)
			return []reconcile.Request{}
		}
		return []reconcile.Request{
			{
				NamespacedName: types.NamespacedName{Namespace: key.Namespace, Name: key.Name},
			},
		}
	})

	// TODO upgrade to controller-runtime 0.11.1 for better log https://github.com/kubernetes-sigs/controller-runtime/pull/1687
	httpRoute := gatewayType.HTTPRoute{}
	utils.AddTypeInformationToObject(scheme, &httpRoute)
	tcpRoute := gatewayType.TCPRoute{}
	utils.AddTypeInformationToObject(scheme, &tcpRoute)
	tlspRoute := gatewayType.TLSRoute{}
	utils.AddTypeInformationToObject(scheme, &tlspRoute)
	udpRoute := gatewayType.UDPRoute{}
	utils.AddTypeInformationToObject(scheme, &udpRoute)

	b = b.Watches(&source.Kind{Type: &httpRoute}, eventhandler, options)
	b = b.Watches(&source.Kind{Type: &tcpRoute}, eventhandler, options)
	b = b.Watches(&source.Kind{Type: &tlspRoute}, eventhandler, options)
	b = b.Watches(&source.Kind{Type: &udpRoute}, eventhandler, options)
	return b
}

func (g *GatewayReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := g.log.WithValues("gateway", request.NamespacedName, "class", g.class)

	log.Info("Reconciling Gateway")

	key := request.NamespacedName

	gateway := &gatewayType.Gateway{}
	err := g.c.Get(g.ctx, key, gateway)
	if errors.IsNotFound(err) {
		log.Info("not found,ignore", "gateway", request.String())
		return reconcile.Result{}, nil
	}
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("get gateway fail %v", err)
	}

	if string(gateway.Spec.GatewayClassName) != g.class {
		log.Info("reconcile a gateway which not belongs to use")
		return reconcile.Result{}, nil
	}

	log.Info("reconcile gateway ", "version", gateway.ResourceVersion, "generation", gateway.Generation)

	allListener, err := ListListenerByClass(ctx, g.c, g.class)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("list listener fail %v", err)
	}

	listenerInGateway := []*Listener{}
	for _, l := range gateway.Spec.Listeners {
		listenerInGateway = append(listenerInGateway, &Listener{Listener: l, gateway: key, status: ListenerStatus{valid: true}})
	}

	log.Info("list listener", "all-len", len(allListener), "ls", len(listenerInGateway))

	g.filteListener(key, listenerInGateway, allListener)

	routes, err := ListRoutesByGateway(ctx, g.c, key)
	if err != nil {
		log.Info("st wrong here", "err", err)
		return reconcile.Result{}, fmt.Errorf("list route fail %v", err)
	}
	log.Info("list route by gateway", "routes-len", len(routes))

	g.filteRoutes(key, routes, listenerInGateway)

	err = g.updateGatewayStatus(gateway, listenerInGateway)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("update gateway status fail %v", err)
	}

	err = g.updateRouteStatus(routes)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("update route status fail %v", err)
	}

	return reconcile.Result{}, nil
}

func (g *GatewayReconciler) filteListener(key client.ObjectKey, ls []*Listener, allls []*Listener) {
	for _, f := range g.invalidListenerfilter {
		f.FilteListener(key, ls, allls)
	}
}

func (g *GatewayReconciler) filteRoutes(gateway client.ObjectKey, routes []*Route, ls []*Listener) {
	log := g.log.WithName("filteoute").WithValues("gateway", gateway)
	// filte route which sectionName is nil
	for _, r := range routes {
		refs := r.route.GetSpec().ParentRefs
		for _, ref := range refs {
			if IsRefToGateway(ref, gateway) && ref.SectionName == nil {
				r.invalidSectionName(ref, "sectionName could not be nil")
			}
		}
	}

	lsMap := map[string]*Listener{}
	for _, l := range ls {
		key := fmt.Sprintf("%s/%s/%s", l.gateway.Namespace, l.gateway.Name, l.Name)
		lsMap[key] = l
	}

	for _, r := range routes {
		for _, ref := range r.route.GetSpec().ParentRefs {
			key := fmt.Sprintf("%s/%s/%s", *ref.Namespace, ref.Name, *ref.SectionName)
			ls, ok := lsMap[key]
			if !ok {
				r.invalidSectionName(ref, fmt.Sprintf("could not find this sectionName %v", key))
				continue
			}
			// route are acceptped by default, unless some filter reject it.
			r.accept(ref)
			for _, f := range g.invalidRoutefilter {
				valid := f.FilteRoute(ref, r, ls)
				log.Info("filte route", "name", f.Name(), "valid", valid, "route", GetObjectKey(r.route), "ls", ls.Name)
			}
		}
	}

	// calculate attachedRoutes
	for _, r := range routes {
		for _, s := range r.status {
			if !s.accept {
				continue
			}
			ref := s.ref
			key := fmt.Sprintf("%s/%s/%s", *ref.Namespace, ref.Name, *ref.SectionName)
			ls, ok := lsMap[key]
			if !ok {
				log.Error(fmt.Errorf("impossable, could not find listener"), "ref", ref)
				continue
			}
			ls.status.attachedRoutes++
		}
	}
}

func (g *GatewayReconciler) updateGatewayStatus(gateway *gatewayType.Gateway, ls []*Listener) error {
	ips, err := getAlbPodIp(g.ctx, g.c, config.GetNs(), config.GetDomain(), config.GetAlbName())
	if err != nil {
		return fmt.Errorf("get pod ip fail err %v", err)
	}
	address := []gatewayType.GatewayAddress{}
	ipType := gatewayType.IPAddressType
	for _, ip := range ips {
		address = append(address, gatewayType.GatewayAddress{Type: &ipType, Value: ip})
	}

	gateway.Status.Addresses = address
	lsstatusList := []gatewayType.ListenerStatus{}
	valid := true
	for _, l := range ls {
		if !l.status.valid {
			valid = false
		}
		log := g.log.WithValues("gateway", l.gateway, "listener", l.Name)
		conditions := l.status.toConditions(gateway)
		log.V(2).Info("conditions of listener", "conditions", conditions)
		lsstatus := gatewayType.ListenerStatus{
			Name:           l.Name,
			SupportedKinds: generateSupportKind(l.Protocol, g.supportKind),
			AttachedRoutes: l.status.attachedRoutes,
			Conditions:     conditions,
		}
		lsstatusList = append(lsstatusList, lsstatus)
	}
	gateway.Status.Listeners = lsstatusList
	if valid {
		gateway.Status.Conditions = []metav1.Condition{
			{

				Type:               string(gatewayType.GatewayConditionReady),
				Status:             metav1.ConditionTrue,
				Reason:             string(gatewayType.GatewayReasonReady),
				LastTransitionTime: metav1.Now(),
				ObservedGeneration: gateway.Generation,
			},
		}
	} else {
		gateway.Status.Conditions = []metav1.Condition{
			{

				Type:               string(gatewayType.GatewayConditionReady),
				Status:             metav1.ConditionFalse,
				Reason:             string(gatewayType.GatewayReasonListenersNotReady),
				Message:            "one or more listener not ready",
				LastTransitionTime: metav1.Now(),
				ObservedGeneration: gateway.Generation,
			},
		}
	}
	oldVersion := gateway.ResourceVersion
	err = g.c.Status().Update(g.ctx, gateway)
	newVersion := gateway.ResourceVersion
	g.log.V(2).Info("update gateway status", "err", err, "gateway", client.ObjectKeyFromObject(gateway), "oldVersion", oldVersion, "newVersion", newVersion)
	if err != nil {
		return err
	}
	return nil
}

func (g *GatewayReconciler) updateRouteStatus(rs []*Route) error {
	updateRoute := func(ps []gatewayType.RouteParentStatus, r *Route) []gatewayType.RouteParentStatus {
		// TODO a route could attach to different gateway controll by different controller. we should not reset route status directly.
		ret := []gatewayType.RouteParentStatus{}

		for _, p := range r.status {
			status := metav1.ConditionTrue
			if !p.accept {
				status = metav1.ConditionFalse
			}
			ret = append(ret, gatewayType.RouteParentStatus{
				ParentRef:      p.ref,
				ControllerName: gatewayType.GatewayController(g.controllerName),
				Conditions: []metav1.Condition{
					{
						Type:               "Ready",
						Status:             status,
						Reason:             string(gatewayType.ListenerReasonReady),
						LastTransitionTime: metav1.Now(),
						ObservedGeneration: r.route.GetObject().GetGeneration(),
						Message:            p.msg,
					},
				},
			})
		}
		return ret
	}

	for _, r := range rs {
		log := g.log.WithValues("route", "route", GetObjectKey(r.route))
		status, err := UpdateRouteStatus(r.route, func(ss []gatewayType.RouteParentStatus) []gatewayType.RouteParentStatus {
			s := updateRoute(ss, r)
			return s
		})
		if err != nil {
			log.Error(err, "update route status fail")
			return err
		}
		log.V(2).Info("update route status", "route", GetObjectKey(r.route), "status", status)
		err = g.c.Status().Update(g.ctx, r.route.GetObject())
		if err != nil {
			return err
		}
	}
	return nil
}
