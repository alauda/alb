package main

import (
	"os"
	"time"

	"alauda.io/alb2/pkg/operator/config"
	"alauda.io/alb2/pkg/operator/controllers"
	"alauda.io/alb2/utils/log"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	//+kubebuilder:scaffold:imports
)

var (
	scheme = runtime.NewScheme()
)

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

	retryPeriod := time.Duration(12 * time.Second)
	renewDeadline := time.Duration(40 * time.Second)
	leaseDuration := time.Duration(60 * time.Second)
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                        scheme,
		MetricsBindAddress:            metricsAddr,
		Port:                          9443,
		HealthProbeBindAddress:        probeAddr,
		LeaderElection:                enableLeaderElection,
		LeaderElectionID:              "alb-operator",
		LeaderElectionReleaseOnCancel: true,
		LeaseDuration:                 &leaseDuration,
		RenewDeadline:                 &renewDeadline,
		RetryPeriod:                   &retryPeriod,
	})

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
	if err = (&controllers.ALB2Reconciler{
		Client:     mgr.GetClient(),
		OperatorCf: operator,
		Log:        l,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ALB2")
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

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
