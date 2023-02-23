package ctl

import (
	"context"
	"fmt"
	"time"

	"alauda.io/alb2/config"
	. "alauda.io/alb2/gateway"
	"alauda.io/alb2/utils"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
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
	gt "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

type GatewayReconciler struct {
	ctx                   context.Context
	controllerName        string
	c                     client.Client
	log                   logr.Logger
	invalidListenerfilter []ListenerFilter
	invalidRoutefilter    []RouteFilter
	supportKind           map[string][]string
	cfg                   config.GatewayCfg
}

type ListenerFilter interface {
	FilteListener(gateway client.ObjectKey, ls []*Listener, allls []*Listener)
	Name() string
}

type RouteFilter interface {
	// 1. you must call route.unAllowRoute(ref,msg) by youself if route couldnot not match
	// 2. you must return false if route is not match
	FilteRoute(ref gt.ParentRef, route *Route, ls *Listener) bool
	Name() string
}

func NewGatewayReconciler(ctx context.Context, c client.Client, log logr.Logger, cfg config.GatewayCfg) GatewayReconciler {

	commonFilter := CommonFiliter{log: log, c: c, ctx: ctx}
	hostNameFilter := HostNameFilter{log: log}

	listenerFilter := []ListenerFilter{
		&commonFilter,
	}
	routeFilter := []RouteFilter{
		&commonFilter,
		&hostNameFilter,
	}

	return GatewayReconciler{
		c:                     c,
		log:                   log,
		ctx:                   ctx,
		controllerName:        GetControllerName(),
		invalidListenerfilter: listenerFilter,
		invalidRoutefilter:    routeFilter,
		cfg:                   cfg,
	}
}

func (g *GatewayReconciler) SetupWithManager(mgr ctrl.Manager) error {
	b := ctrl.NewControllerManagedBy(mgr).
		For(&gt.Gateway{}, ctrlBuilder.WithPredicates(predicate.NewPredicateFuncs(g.filterSelectedGateway))).
		WithEventFilter(predicate.GenerationChangedPredicate{})

	b = g.watchRoutes(b)
	// default rate limit should be enough for use.
	b = b.WithOptions(controller.Options{RateLimiter: workqueue.DefaultControllerRateLimiter()})

	return b.Complete(g)
}

func (g *GatewayReconciler) filterSelectedGateway(o client.Object) (ret bool) {
	defer func() {
		g.log.V(5).Info("filter gateway", "kind", o.GetObjectKind(), "key", client.ObjectKeyFromObject(o), "ret", ret)
	}()
	sel := g.cfg.GatewaySelector
	if sel.GatewayClass != nil {
		class := *sel.GatewayClass
		switch g := o.(type) {
		case *gt.Gateway:
			return string(g.Spec.GatewayClassName) == class
		}
		return false
	}
	if sel.GatewayName != nil {
		key := *sel.GatewayName
		return client.ObjectKeyFromObject(o) == key
	}
	return false
}

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
	httpRoute := gt.HTTPRoute{}
	utils.AddTypeInformationToObject(scheme, &httpRoute)
	tcpRoute := gt.TCPRoute{}
	utils.AddTypeInformationToObject(scheme, &tcpRoute)
	tlspRoute := gt.TLSRoute{}
	utils.AddTypeInformationToObject(scheme, &tlspRoute)
	udpRoute := gt.UDPRoute{}
	utils.AddTypeInformationToObject(scheme, &udpRoute)

	b = b.Watches(&source.Kind{Type: &httpRoute}, eventhandler, options)
	b = b.Watches(&source.Kind{Type: &tcpRoute}, eventhandler, options)
	b = b.Watches(&source.Kind{Type: &tlspRoute}, eventhandler, options)
	b = b.Watches(&source.Kind{Type: &udpRoute}, eventhandler, options)
	return b
}

func (g *GatewayReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := g.log.WithValues("gateway", request.NamespacedName, "id", g.cfg.String())
	log.Info("Reconciling Gateway", "gateway", request.String())

	key := request.NamespacedName

	gateway := &gt.Gateway{}
	err := g.c.Get(g.ctx, key, gateway)
	if errors.IsNotFound(err) {
		log.Info("not found,ignore", "gateway", request.String())
		return reconcile.Result{}, nil
	}
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("get gateway fail %v", err)
	}

	log.Info("reconcile gateway ", "version", gateway.ResourceVersion, "generation", gateway.Generation)
	allListener, err := ListListener(ctx, g.c, g.cfg.GatewaySelector)
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
	log.Info("list route by gateway", "key", key, "routes-len", len(routes))

	g.filteRoutes(key, routes, listenerInGateway)

	ip, getIpErr := g.GetGatewayIp(gateway)
	if getIpErr != nil {
		ip = ""
	}
	err = g.updateGatewayStatus(gateway, listenerInGateway, ip)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("update gateway status fail %v", err)
	}

	err = g.updateRouteStatus(routes)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("update route status fail %v", err)
	}
	// retry to sync gateway ip
	if getIpErr != nil {
		return reconcile.Result{RequeueAfter: time.Second * 3}, nil
	}
	return reconcile.Result{}, nil
}

func (g *GatewayReconciler) filteListener(key client.ObjectKey, ls []*Listener, allls []*Listener) {
	for _, f := range g.invalidListenerfilter {
		f.FilteListener(key, ls, allls)
	}
}

func (g *GatewayReconciler) filteRoutes(gateway client.ObjectKey, routes []*Route, ls []*Listener) {
	log := g.log.WithName("filteroute").WithValues("gateway", gateway)
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
		log.V(5).Info("init lsmap", "key", key)
		lsMap[key] = l
	}

	for _, r := range routes {
		for _, ref := range r.route.GetSpec().ParentRefs {
			if !IsRefToGateway(ref, gateway) {
				continue
			}
			key := fmt.Sprintf("%s/%s/%s", *ref.Namespace, ref.Name, *ref.SectionName)
			ls, ok := lsMap[key]
			if !ok {
				log.V(5).Info("could not find this route ref", "key", key)
				r.invalidSectionName(ref, fmt.Sprintf("could not find this sectionName %v", key))
				continue
			}
			// route are acceptped by default, unless some filter reject it.
			r.accept(ref)
			for _, f := range g.invalidRoutefilter {
				valid := f.FilteRoute(ref, r, ls)
				if !valid {
					log.Info("find a invalid route", "name", f.Name(), "valid", valid, "route", GetObjectKey(r.route), "ls", ls.Name)
					continue
				}
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
				log.Error(fmt.Errorf("impossible, could not find listener"), "ref", ref)
				continue
			}
			ls.status.attachedRoutes++
		}
	}
}

func (g *GatewayReconciler) updateGatewayStatus(gateway *gt.Gateway, ls []*Listener, ip string) error {
	address := []gt.GatewayAddress{}
	if ip != "" {
		ipType := gt.IPAddressType
		address = []gt.GatewayAddress{{Type: &ipType, Value: ip}}
	}

	gateway.Status.Addresses = address
	lsstatusList := []gt.ListenerStatus{}
	valid := true
	for _, l := range ls {
		if !l.status.valid {
			valid = false
		}
		log := g.log.WithValues("gateway", l.gateway, "listener", l.Name)
		conditions := l.status.toConditions(gateway)
		log.V(2).Info("conditions of listener", "conditions", conditions)
		lsstatus := gt.ListenerStatus{
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

				Type:               string(gt.GatewayConditionReady),
				Status:             metav1.ConditionTrue,
				Reason:             string(gt.GatewayReasonReady),
				LastTransitionTime: metav1.Now(),
				ObservedGeneration: gateway.Generation,
			},
		}
	} else {
		gateway.Status.Conditions = []metav1.Condition{
			{

				Type:               string(gt.GatewayConditionReady),
				Status:             metav1.ConditionFalse,
				Reason:             string(gt.GatewayReasonListenersNotReady),
				Message:            "one or more listener not ready",
				LastTransitionTime: metav1.Now(),
				ObservedGeneration: gateway.Generation,
			},
		}
	}
	oldVersion := gateway.ResourceVersion
	err := g.c.Status().Update(g.ctx, gateway)
	newVersion := gateway.ResourceVersion
	g.log.V(2).Info("update gateway status", "err", err, "gateway", client.ObjectKeyFromObject(gateway), "oldVersion", oldVersion, "newVersion", newVersion)
	if err != nil {
		return err
	}
	return nil
}

func (g *GatewayReconciler) GetGatewayIp(gw *gt.Gateway) (string, error) {
	// get ip from alb.
	g.log.Info("get gateway ip", "mode", g.cfg.Mode)
	if g.cfg.Mode == config.GatewayClass {
		ret, err := getAlbAddress(g.ctx, g.c, config.GetNs(), config.GetAlbName())
		g.log.Info("get gateway ip", "ret", ret, "err", err)
		return ret, err
	}
	// get ip from lb svc.
	if g.cfg.Mode == config.Gateway {
		return getLbSvcLbIp(g.ctx, g.c, config.GetNs(), config.GetAlbName()+"-tcp")
	}

	return "", fmt.Errorf("invalid gaterway cfg %v", g.cfg)
}

func (g *GatewayReconciler) updateRouteStatus(rs []*Route) error {
	// we must keep condition which ref to other gateway.
	updateRoute := func(origin []gt.RouteParentStatus, r *Route) []gt.RouteParentStatus {
		psMap := map[string]gt.RouteParentStatus{}
		for _, ss := range origin {
			psMap[RefsToString(ss.ParentRef)] = ss
		}

		for _, p := range r.status {
			key := RefsToString(p.ref)
			status := metav1.ConditionTrue
			if !p.accept {
				status = metav1.ConditionFalse
			}
			psMap[key] = gt.RouteParentStatus{
				ParentRef:      p.ref,
				ControllerName: gt.GatewayController(g.controllerName),
				Conditions: []metav1.Condition{
					{
						Type:               "Ready",
						Status:             status,
						Reason:             string(gt.ListenerReasonReady),
						LastTransitionTime: metav1.Now(),
						ObservedGeneration: r.route.GetObject().GetGeneration(),
						Message:            p.msg,
					},
				},
			}
		}

		ret := []gt.RouteParentStatus{}
		for _, s := range psMap {
			ret = append(ret, s)
		}
		return ret
	}

	for _, r := range rs {
		log := g.log.WithValues("route", "route", GetObjectKey(r.route))
		status, err := UpdateRouteStatus(r.route, func(ss []gt.RouteParentStatus) []gt.RouteParentStatus {
			return updateRoute(ss, r)
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
