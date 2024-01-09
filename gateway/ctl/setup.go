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
	gv1 "sigs.k8s.io/gateway-api/apis/v1"
	gv1a2t "sigs.k8s.io/gateway-api/apis/v1alpha2"

	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

var scheme = runtime.NewScheme()

func init() {
	_ = k8sScheme.AddToScheme(scheme)
	_ = gv1a2t.AddToScheme(scheme)
	_ = gv1.AddToScheme(scheme)
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
	cfg := config.GetConfig()
	restCfg, err := driver.GetKubeCfg(cfg.K8s)
	if err != nil {
		return err
	}
	gatewayCfg := cfg.GetGatewayCfg()
	l.Info("gateway cfg", "cfg", gatewayCfg)

	mgr, err := ctrl.NewManager(restCfg, ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: "0",
		},
		LeaderElection:   false, // disable leader election. we use alb's leader election
		LeaderElectionID: "",
	})
	if err != nil {
		return err
	}

	g := NewGatewayReconciler(ctx, mgr.GetClient(), l.WithName("gateway"), cfg)
	err = g.SetupWithManager(mgr)
	if err != nil {
		return err
	}
	if gatewayCfg.Mode == albv2Type.GatewayModeShared {
		gc := NewGatewayClassReconciler(ctx, mgr.GetClient(), l.WithName("gatewayclass"))
		err = gc.SetupWithManager(mgr)
		if err != nil {
			return err
		}
	}

	err = mgr.Start(ctx)
	return err
}
