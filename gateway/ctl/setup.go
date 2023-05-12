package ctl

import (
	"context"

	"alauda.io/alb2/config"
	"alauda.io/alb2/driver"
	g "alauda.io/alb2/gateway"
	albType "alauda.io/alb2/pkg/apis/alauda/v1"
	albv2Type "alauda.io/alb2/pkg/apis/alauda/v2beta1"
	. "alauda.io/alb2/utils/log"
	"k8s.io/apimachinery/pkg/runtime"
	k8sScheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	gv1a2t "sigs.k8s.io/gateway-api/apis/v1alpha2"
	gv1b1t "sigs.k8s.io/gateway-api/apis/v1beta1"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	_ = k8sScheme.AddToScheme(scheme)
	_ = gv1a2t.AddToScheme(scheme)
	_ = gv1b1t.AddToScheme(scheme)
	_ = albType.AddToScheme(scheme)
	_ = albv2Type.AddToScheme(scheme)
}

func Run(ctx context.Context) {
	err := StartGatewayController(ctx)
	if err != nil {
		panic(err)
	}
}

func StartGatewayController(ctx context.Context) error {
	l := L().WithName(g.ALB_GATEWAY_CONTROLLER)
	ctrl.SetLogger(l)
	restCfg, err := driver.GetKubeCfg()
	if err != nil {
		return err
	}
	gatewayCfg := config.GetConfig().GetGatewayCfg()
	l.Info("gateway cfg", "cfg", gatewayCfg)

	mgr, err := ctrl.NewManager(restCfg, ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: "0",   // disable metrics
		LeaderElection:     false, // disable leader election. we use alb's leader election
		LeaderElectionID:   "",
	})

	if err != nil {
		return err
	}

	g := NewGatewayReconciler(ctx, mgr.GetClient(), l.WithName("gateway"), gatewayCfg)
	err = g.SetupWithManager(mgr)
	if err != nil {
		return err
	}
	if gatewayCfg.Mode == config.GatewayClass {
		gc := NewGatewayClassReconciler(ctx, mgr.GetClient(), l.WithName("gatewayclass"))
		err = gc.SetupWithManager(mgr)
		if err != nil {
			return err
		}
	}

	err = mgr.Start(ctx)
	return err
}
