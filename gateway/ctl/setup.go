package ctl

import (
	"context"

	"alauda.io/alb2/driver"
	g "alauda.io/alb2/gateway"
	. "alauda.io/alb2/utils/log"
	"k8s.io/apimachinery/pkg/runtime"
	k8sScheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	gatewayType "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	_ = k8sScheme.AddToScheme(scheme)
	_ = gatewayType.AddToScheme(scheme)
}

func Run(ctx context.Context) {
	err := StartGatewayController(ctx)
	if err != nil {
		panic(err)
	}
}

func StartGatewayController(ctx context.Context) error {
	ctrl.SetLogger(L().WithName(g.ALB_GATEWAY_CONTROLLER))
	restCfg, err := driver.GetKubeCfg()
	if err != nil {
		return err
	}

	mgr, err := ctrl.NewManager(restCfg, ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: "0",   // disable metrics
		LeaderElection:     false, // disable leader election. we use alb's leader election
		LeaderElectionID:   "",
	})

	if err != nil {
		return err
	}

	g := NewGatewayReconciler(ctx, mgr.GetClient(), ctrl.Log.WithName("gateway"))
	err = g.SetupWithManager(mgr)
	if err != nil {
		return err
	}
	gc := NewGatewayClassReconciler(ctx, mgr.GetClient(), ctrl.Log.WithName("gatewayclass"))
	err = gc.SetupWithManager(mgr)
	if err != nil {
		return err
	}

	err = mgr.Start(ctx)
	return err
}
