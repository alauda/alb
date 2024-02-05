package ctl

import (
	"context"
	"fmt"
	"reflect"
	"time"

	alb2v2 "alauda.io/alb2/pkg/apis/alauda/v2beta1"

	"alauda.io/alb2/config"
	. "alauda.io/alb2/gateway"
	u "alauda.io/alb2/utils"
	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	"github.com/samber/lo"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/kube-openapi/pkg/util/sets"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlBuilder "sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gv1 "sigs.k8s.io/gateway-api/apis/v1"
)

type GatewayReconciler struct {
	ctx                   context.Context
	controllerName        string
	c                     client.Client
	log                   logr.Logger
	invalidListenerfilter []ListenerFilter
	invalidRoutefilter    []RouteFilter
	supportKind           map[string][]string
	albcfg                *config.Config
	cfg                   config.GatewayCfg
}

type ListenerFilter interface {
	FilteListener(gateway client.ObjectKey, ls []*Listener, allls []*Listener)
	Name() string
}

type RouteFilter interface {
	// 1. you must call route.unAllowRoute(ref,msg) by yourself if route couldnt not match
	// 2. you must return false if route is not match
	FilteRoute(ref gv1.ParentReference, route *Route, ls *Listener) bool
	Name() string
}

func NewGatewayReconciler(ctx context.Context, c client.Client, log logr.Logger, cfg *config.Config) GatewayReconciler {
	commonFilter := CommonFiliter{log: log, c: c, ctx: ctx}
	hostNameFilter := HostNameFilter{log: log}
	gc := cfg.GetGatewayCfg()
	reservedPortFilter := NewReservedPortFilter(log, []int{gc.ReservedPort, 1936, 11782})

	listenerFilter := []ListenerFilter{
		&commonFilter,
	}
	routeFilter := []RouteFilter{
		&commonFilter,
		&hostNameFilter,
		&reservedPortFilter,
	}

	return GatewayReconciler{
		c:                     c,
		log:                   log,
		ctx:                   ctx,
		controllerName:        GetControllerName(),
		invalidListenerfilter: listenerFilter,
		invalidRoutefilter:    routeFilter,
		supportKind:           SUPPORT_KIND_MAP,
		cfg:                   gc,
		albcfg:                cfg,
	}
}

func (g *GatewayReconciler) SetupWithManager(mgr ctrl.Manager) error {
	b := ctrl.NewControllerManagedBy(mgr).
		For(&gv1.Gateway{}, ctrlBuilder.WithPredicates(predicate.NewPredicateFuncs(g.filterSelectedGateway)))

	b = g.watchRoutes(b)
	b = g.watchAlb(b)
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
		case *gv1.Gateway:
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

func (g *GatewayReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := g.log.WithValues("gateway", request.NamespacedName, "id", g.cfg.String())
	log.Info("Reconciling Gateway", "gateway", request.String())

	key := request.NamespacedName

	gateway := &gv1.Gateway{}
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

	alb, getAlbErr := g.GetGatewayAlb(gateway)
	if getAlbErr != nil {
		log.Error(err, "get gateway alb fail")
	}
	requene, msg, err := g.updateGatewayStatus(gateway, listenerInGateway, alb)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("update gateway status fail %v", err)
	}

	err = g.updateRouteStatus(routes)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("update route status fail %v", err)
	}
	// retry to sync gateway ip
	if getAlbErr != nil {
		return reconcile.Result{RequeueAfter: time.Second * 3}, nil
	}
	if requene {
		log.Info("requene", "cause", msg)
		return reconcile.Result{RequeueAfter: time.Second * 10}, nil
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

func (g *GatewayReconciler) updateGatewayStatus(gateway *gv1.Gateway, ls []*Listener, alb *alb2v2.ALB2) (requene bool, msg string, err error) {
	origin := gateway.DeepCopy()
	address := []gv1.GatewayStatusAddress{}
	albaddress := alb.GetAllAddress()
	ips, hosts := u.ParseAddressList(albaddress)

	for _, host := range hosts {
		if host == "" {
			continue
		}
		hostType := gv1.HostnameAddressType
		address = append(address, gv1.GatewayStatusAddress{Type: &hostType, Value: host})
	}
	for _, ip := range ips {
		if ip == "" {
			continue
		}
		ipType := gv1.IPAddressType
		address = append(address, gv1.GatewayStatusAddress{Type: &ipType, Value: ip})
	}
	gateway.Status.Addresses = address
	lsstatusList := []gv1.ListenerStatus{}
	listenerValid := true
	for _, l := range ls {
		if !l.status.valid {
			listenerValid = false
		}
		log := g.log.WithValues("gateway", l.gateway, "listener", l.Name)

		conditions := l.status.toConditions(gateway)
		log.V(2).Info("conditions of listener", "conditions", conditions)
		lsstatus := gv1.ListenerStatus{
			Name:           l.Name,
			AttachedRoutes: l.status.attachedRoutes,
			SupportedKinds: []gv1.RouteGroupKind{},
			Conditions:     conditions,
		}
		if !l.status.allKindInvalid {
			lsstatus.SupportedKinds = generateSupportKind(l.Protocol, g.supportKind)
		}
		lsstatusList = append(lsstatusList, lsstatus)
	}
	gateway.Status.Listeners = lsstatusList
	albReady := alb.Status.State == alb2v2.ALB2StateRunning
	allReady := true
	acceptCondition := metav1.Condition{
		Type:               string(gv1.GatewayConditionAccepted),
		LastTransitionTime: metav1.Now(),
		ObservedGeneration: gateway.Generation,
	}
	if albReady {
		acceptCondition.Status = metav1.ConditionTrue
		acceptCondition.Reason = string(gv1.GatewayReasonReady)
		acceptCondition.Message = ""
	} else {
		allReady = false
		acceptCondition.Status = metav1.ConditionUnknown
		acceptCondition.Reason = string(gv1.GatewayReasonPending)
		acceptCondition.Message = alb.Status.Reason
	}

	var program *metav1.Condition
	var ready *metav1.Condition
	if listenerValid {
		ready = &metav1.Condition{
			Type:               string(gv1.GatewayConditionReady),
			Status:             metav1.ConditionTrue,
			Reason:             string(gv1.GatewayReasonAccepted),
			LastTransitionTime: metav1.Now(),
			ObservedGeneration: gateway.Generation,
		}
	} else {
		ready = &metav1.Condition{
			Type:               string(gv1.GatewayConditionReady),
			Status:             metav1.ConditionFalse,
			Reason:             string(gv1.GatewayReasonListenersNotReady),
			Message:            "one or more listener not ready",
			LastTransitionTime: metav1.Now(),
			ObservedGeneration: gateway.Generation,
		}
	}
	if len(albaddress) != 0 {
		program = &metav1.Condition{
			Type:               string(gv1.GatewayConditionProgrammed),
			Status:             metav1.ConditionTrue,
			Reason:             string(gv1.GatewayReasonProgrammed),
			LastTransitionTime: metav1.Now(),
			ObservedGeneration: gateway.Generation,
		}
	} else {
		ready = &metav1.Condition{
			Type:               string(gv1.GatewayConditionReady),
			Status:             metav1.ConditionFalse,
			Reason:             string(gv1.GatewayReasonAddressNotAssigned),
			LastTransitionTime: metav1.Now(),
			ObservedGeneration: gateway.Generation,
		}
		allReady = false
	}

	conditions := []metav1.Condition{
		acceptCondition,
	}

	conditions = append(conditions, *ready)
	if program != nil {
		conditions = append(conditions, *program)
	}
	gateway.Status.Conditions = conditions

	if sameGatewayStatus(origin.Status, gateway.Status) && allReady {
		g.log.Info("gateway status same ignore")
		return
	}
	g.log.Info("status", "condition", "diff", cmp.Diff(origin.Status, gateway.Status))
	oldVersion := gateway.ResourceVersion
	err = g.c.Status().Update(g.ctx, gateway)
	newVersion := gateway.ResourceVersion
	g.log.Info("update gateway status", "err", err, "gateway", client.ObjectKeyFromObject(gateway), "oldVersion", oldVersion, "newVersion", newVersion)
	if err != nil {
		return false, "", err
	}
	if !allReady {
		return true, "not all ready retry", nil
	}
	return false, "", err
}

func (g *GatewayReconciler) GetGatewayAlb(gw *gv1.Gateway) (*alb2v2.ALB2, error) {
	// get ip from alb.
	ns, name := config.GetConfig().GetAlbNsAndName()
	return getAlb(g.ctx, g.c, ns, name)
}

func (g *GatewayReconciler) updateRouteStatus(rs []*Route) error {
	// we must keep condition which ref to other gateway.
	updateRoute := func(origin []gv1.RouteParentStatus, r *Route) []gv1.RouteParentStatus {
		psMap := map[string]gv1.RouteParentStatus{}
		for _, ss := range origin {
			psMap[RefsToString(ss.ParentRef)] = ss
		}

		for _, p := range r.status {
			key := RefsToString(p.ref)
			status := metav1.ConditionTrue
			if !p.accept {
				status = metav1.ConditionFalse
			}
			reason := string(gv1.ListenerReasonReady)
			if p.reason != "" {
				reason = p.reason
			}
			psMap[key] = gv1.RouteParentStatus{
				ParentRef:      p.ref,
				ControllerName: gv1.GatewayController(g.controllerName),
				Conditions: []metav1.Condition{
					{
						Type:               string(gv1.ListenerConditionReady),
						Status:             status,
						Reason:             reason,
						LastTransitionTime: metav1.Now(),
						ObservedGeneration: r.route.GetObject().GetGeneration(),
						Message:            p.msg,
					},
				},
			}
		}

		ret := []gv1.RouteParentStatus{}
		for _, s := range psMap {
			ret = append(ret, s)
		}
		return ret
	}

	// NOTE: alb现在是根据route的状态来生成policy的。所以必须要处理完所有的route。不能因为一个route出错而提前退出
	for _, r := range rs {
		log := g.log.WithValues("route", "route", GetObjectKey(r.route))
		origin, err := GetRouteStatus(r.route)
		if err != nil {
			log.Error(err, "invalid route")
			continue
		}
		newStatus, err := UpdateRouteStatus(r.route, func(ss []gv1.RouteParentStatus) []gv1.RouteParentStatus {
			return updateRoute(ss, r)
		})
		if err != nil {
			log.Error(err, "update route status fail")
			continue
		}
		if SameStatus(origin, newStatus) {
			log.Info("same status ignore")
			continue
		}
		log.Info("update route status", "route", GetObjectKey(r.route), "status", newStatus, "diff", cmp.Diff(origin, newStatus))
		err = g.c.Status().Update(g.ctx, r.route.GetObject())
		if err != nil {
			log.Error(err, "k8s update route status fail")
			continue
		}
	}
	return nil
}

func SameCondition(left []metav1.Condition, right []metav1.Condition) bool {
	if len(left) != len(right) {
		return false
	}
	for i, oc := range left {
		nc := right[i]
		n := metav1.Now()
		oc.LastTransitionTime = n
		nc.LastTransitionTime = n
		if !reflect.DeepEqual(oc, nc) {
			return false
		}
	}
	return true
}

func sameAddress(origin []gv1.GatewayStatusAddress, latest []gv1.GatewayStatusAddress) bool {
	orginset := sets.NewString(lo.Map(origin, func(s gv1.GatewayStatusAddress, _ int) string { return s.Value })...)
	statusset := sets.NewString(lo.Map(latest, func(s gv1.GatewayStatusAddress, _ int) string { return s.Value })...)
	return orginset.Equal(statusset)
}

func sameGatewayStatus(origin gv1.GatewayStatus, latest gv1.GatewayStatus) bool {
	return SameCondition(origin.Conditions, latest.Conditions) &&
		sameAddress(origin.Addresses, latest.Addresses) &&
		sameListenerStatus(origin.Listeners, latest.Listeners)
}

func sameListenerStatus(origin []gv1.ListenerStatus, latest []gv1.ListenerStatus) bool {
	tomap := func(ls []gv1.ListenerStatus) map[string]gv1.ListenerStatus {
		m := map[string]gv1.ListenerStatus{}
		for _, s := range ls {
			m[string(s.Name)] = s
		}
		return m
	}
	originMap := tomap(origin)
	newMap := tomap(latest)
	if len(originMap) != len(newMap) {
		return false
	}
	for k, o := range originMap {
		n, ok := newMap[k]
		if !ok {
			return false
		}

		if !(SameCondition(o.Conditions, n.Conditions) && o.AttachedRoutes == n.AttachedRoutes && reflect.DeepEqual(o.SupportedKinds, n.SupportedKinds)) {
			return false
		}
	}
	return true
}

func SameStatus(origin []gv1.RouteParentStatus, latest []gv1.RouteParentStatus) bool {
	if len(origin) != len(latest) {
		return false
	}
	for i, oc := range origin {
		nc := latest[i]
		now := metav1.Now()
		for i := range oc.Conditions {
			oc.Conditions[i].LastTransitionTime = now
		}
		for i := range nc.Conditions {
			nc.Conditions[i].LastTransitionTime = now
		}
		if !reflect.DeepEqual(oc, nc) {
			return false
		}
	}
	return true
}
