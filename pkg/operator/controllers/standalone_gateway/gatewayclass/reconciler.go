package gatewayclass

import (
	"context"
	"fmt"

	"alauda.io/alb2/pkg/operator/config"
	. "alauda.io/alb2/pkg/operator/controllers/types"
	. "alauda.io/alb2/pkg/operator/toolkit"

	"github.com/go-logr/logr"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrcli "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	gv1b1t "sigs.k8s.io/gateway-api/apis/v1beta1"
)

type GatewayClassReconciler struct {
	cli        ctrcli.Client
	OperatorCf config.OperatorCfg
	Log        logr.Logger
}

func NewGatewayClassReconciler(cli ctrcli.Client, cfg config.OperatorCfg, log logr.Logger) *GatewayClassReconciler {
	return &GatewayClassReconciler{
		cli:        cli,
		OperatorCf: cfg,
		Log:        log,
	}
}

func (r *GatewayClassReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Log.Info("set up gatewayclass reconcile")
	return ctrl.NewControllerManagedBy(mgr).
		For(&gv1b1t.GatewayClass{}).Complete(r)
}

func (r *GatewayClassReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	res, err := r.reconcile(ctx, req)
	if err != nil {
		r.Log.Error(err, "reconcile fail")
	}
	return res, err
}

func (r *GatewayClassReconciler) reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := r.Log
	l.Info("reconcile gatewayclass", "class", req)
	gc := &gv1b1t.GatewayClass{}
	err := r.cli.Get(ctx, ctrcli.ObjectKey{Name: req.Name}, gc)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}
	l.Info("reconcile gatewayclass", "class", PrettyCr(gc))
	ctlName := fmt.Sprintf(FMT_STAND_ALONE_GATEWAY_CLASS_CTL_NAME, r.OperatorCf.BaseDomain)
	if gc.Spec.ControllerName != gv1b1t.GatewayController(ctlName) {
		l.Info("not our class. ignore")
		return ctrl.Result{}, nil
	}
	if !hasAccept(gc.Status) {
		gc.Status.Conditions = []metav1.Condition{
			{
				LastTransitionTime: metav1.Now(),
				ObservedGeneration: gc.Generation,
				Type:               string(gv1b1t.GatewayClassConditionStatusAccepted),
				Status:             metav1.ConditionTrue,
				Reason:             string(gv1b1t.GatewayClassReasonAccepted),
			},
		}
		err := r.cli.Status().Update(ctx, gc)
		if err != nil {
			return reconcile.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

func hasAccept(ss gv1b1t.GatewayClassStatus) bool {
	for _, c := range ss.Conditions {
		if c.Type == string(gv1b1t.GatewayClassConditionStatusAccepted) && c.Status == metav1.ConditionTrue {
			return true
		}
	}
	return false
}
