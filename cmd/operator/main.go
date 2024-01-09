package main

import (
	"context"
	"os"
	"time"

	rc "alauda.io/alb2/config"
	"alauda.io/alb2/driver"
	"alauda.io/alb2/pkg/operator/config"
	"alauda.io/alb2/pkg/operator/controllers"
	"alauda.io/alb2/utils/log"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	//+kubebuilder:scaffold:imports
)

var scheme = runtime.NewScheme()

func init() {
	controllers.InitScheme(scheme)
}

func main() {
	metricsAddr := ":8080"
	enableLeaderElection := true
	probeAddr := ":8081" // keep it as same as csv.yaml
	l := log.L()
	setupLog := l.WithName("setup")
	ctrl.SetLogger(l)

	retryPeriod := 12 * time.Second
	renewDeadline := 40 * time.Second
	leaseDuration := 60 * time.Second
	restcfg, err := driver.GetKubeCfg(rc.K8sFromEnv())
	if err != nil {
		l.Error(err, "init kube cfg fail")
		panic(err)
	}
	opt := ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: metricsAddr,
		},
		HealthProbeBindAddress:        probeAddr,
		LeaderElection:                enableLeaderElection,
		LeaderElectionID:              "alb-operator",
		LeaderElectionReleaseOnCancel: true,
		LeaseDuration:                 &leaseDuration,
		RenewDeadline:                 &renewDeadline,
		RetryPeriod:                   &retryPeriod,
	}
	if os.Getenv("LEADER_NS") != "" {
		opt.LeaderElectionNamespace = os.Getenv("LEADER_NS")
	}
	mgr, err := ctrl.NewManager(restcfg, opt)
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}
	operator, err := config.OperatorCfgFromEnv()
	if err != nil {
		setupLog.Error(err, "load cfg fail")
		os.Exit(1)
	}
	setupLog.Info("operator cfg", "cfg", operator)
	err = controllers.Setup(mgr, operator, l)
	if err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}
	ctx := ctrl.SetupSignalHandler()
	if err := mgr.Add(manager.RunnableFunc(func(ctx context.Context) error {
		return controllers.StandAloneGatewayClassInit(ctx, operator, mgr.GetClient(), l)
	})); err != nil {
		l.Error(err, "add init runnable fail")
		os.Exit(1)
	}
	setupLog.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
