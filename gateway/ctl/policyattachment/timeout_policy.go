package policyattachment

import (
	"context"

	"github.com/go-logr/logr"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type TimeoutPolicyReconciler struct {
	ctx context.Context
	log logr.Logger
	c   client.Client
}

func NewTimeoutPolicyReconciler(ctx context.Context, c client.Client, log logr.Logger) TimeoutPolicyReconciler {
	return TimeoutPolicyReconciler{
		log: log,
		ctx: ctx,
		c:   c,
	}
}

func (t *TimeoutPolicyReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := t.log.WithValues("timeoutpolicy", request.NamespacedName)
	_ = log
	return reconcile.Result{}, nil
}

func (t *TimeoutPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).Complete(t)
}
