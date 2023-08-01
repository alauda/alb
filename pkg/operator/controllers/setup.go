package controllers

import (
	"alauda.io/alb2/pkg/operator/config"
	"alauda.io/alb2/pkg/operator/controllers/depl"
	g "alauda.io/alb2/pkg/operator/controllers/standalone_gateway/gateway"
	gc "alauda.io/alb2/pkg/operator/controllers/standalone_gateway/gatewayclass"
	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
)

func Setup(mgr ctrl.Manager, cfg config.OperatorCfg, log logr.Logger) error {
	if err := (&depl.ALB2Reconciler{Client: mgr.GetClient(), OperatorCf: cfg, Log: log}).SetupWithManager(mgr); err != nil {
		return err
	}
	if err := (g.NewGatewayReconciler(mgr.GetClient(), cfg, log)).SetupWithManager(mgr); err != nil {
		return err
	}
	if err := (gc.NewGatewayClassReconciler(mgr.GetClient(), cfg, log)).SetupWithManager(mgr); err != nil {
		return err
	}
	return nil
}
