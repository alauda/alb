package ctl

import (
	"context"
	"fmt"

	. "alauda.io/alb2/gateway"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gatewayType "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

type GatewayClassReconciler struct {
	ctx            context.Context
	class          string
	controllerName string
	c              client.Client
	log            logr.Logger
}

func NewGatewayClassReconciler(ctx context.Context, c client.Client, log logr.Logger) GatewayClassReconciler {

	return GatewayClassReconciler{
		c:              c,
		log:            log,
		ctx:            ctx,
		class:          GetClassName(),
		controllerName: GetControllerName(),
	}
}

func (g *GatewayClassReconciler) SetupWithManager(mgr ctrl.Manager) error {
	b := ctrl.NewControllerManagedBy(mgr).
		For(&gatewayType.GatewayClass{}).
		WithEventFilter(predicate.GenerationChangedPredicate{})

	return b.Complete(g)
}

func (g *GatewayClassReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := g.log.WithValues("class", request.Name)
	log.Info("Reconciling GatewayClass")
	if request.Name != g.class {
		return reconcile.Result{}, nil
	}
	class := &gatewayType.GatewayClass{}
	err := g.c.Get(g.ctx, request.NamespacedName, class)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}
	if string(class.Spec.ControllerName) != g.controllerName {
		log.Info("controller not belong us?", "controller", class.Spec.ControllerName)
		err := g.UnknowController(class)
		return reconcile.Result{}, err
	}
	err = g.AcceptClass(class)
	return reconcile.Result{}, err
}

func (g *GatewayClassReconciler) UnknowController(class *gatewayType.GatewayClass) error {
	class.Status.Conditions = []metav1.Condition{
		{
			Type:               string(gatewayType.GatewayClassConditionStatusAccepted),
			Status:             metav1.ConditionFalse,
			Reason:             "InvalidController",
			ObservedGeneration: class.Generation,
			Message:            fmt.Sprintf("controller should be %v instead of %v", g.controllerName, class.Spec.ControllerName),
			LastTransitionTime: metav1.Now(),
		},
	}
	return g.c.Status().Update(g.ctx, class)
}

func (g *GatewayClassReconciler) AcceptClass(class *gatewayType.GatewayClass) error {
	class.Status.Conditions = []metav1.Condition{
		{
			Type:               string(gatewayType.GatewayClassConditionStatusAccepted),
			Status:             metav1.ConditionTrue,
			Reason:             string(gatewayType.GatewayClassReasonAccepted),
			ObservedGeneration: class.Generation,
			LastTransitionTime: metav1.Now(),
		},
	}
	return g.c.Status().Update(g.ctx, class)
}