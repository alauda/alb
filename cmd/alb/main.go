package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"alauda.io/alb2/config"
	ctl "alauda.io/alb2/controller"
	. "alauda.io/alb2/controller/alb"
	"alauda.io/alb2/controller/modules"
	"alauda.io/alb2/controller/state"
	"alauda.io/alb2/driver"
	"alauda.io/alb2/utils/log"

	"github.com/go-logr/logr"
)

func main() {
	defer log.Flush()
	if err := run(); err != nil {
		log.L().Error(err, "run fail")
		log.Flush()
	}
}

func run() error {
	l := log.L().WithName("lifecycle")

	albCfg := config.GetConfig()
	restCfg, err := driver.GetKubeCfg(albCfg.K8s)
	if err != nil {
		l.Error(err, "get rest cfg fail")
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	leCtx, leCancel := context.WithCancel(ctx)
	le := ctl.NewLeaderElection(leCtx, albCfg, restCfg, log.L().WithName("le"))

	a := NewAlb(ctx, restCfg, albCfg, le, log.L())

	go StartSignalLoop(cancel, SignalCallBack{
		OnSigInt: func() {
			l.Info("receive SIGINT(ctrl-c), shutting down")
			cancel()
			os.Exit(0)
		},
		OnSigTerm: func() {
			l.Info("receive SIGTERM(graceful-shutdown), close nginx port")
			state.GetState().SetPhase(modules.PhaseTerminating)
			leCancel()
			l.Info("receive SIGTERM(graceful-shutdown), cancel leader")
			//  could not cancel here. waiting f5 healthcheck to remove this port.
			//  wait nginx close 1936 metrics port
			//  then we could stop alb.
		},
	}, log.L().WithName("signal"))

	if err = a.Start(); err != nil {
		l.Error(err, "alb run fail")
		return err
	}
	l.Info("graceful shutdown")
	return nil
}

type SignalCallBack struct {
	OnSigInt  func()
	OnSigTerm func()
}

func StartSignalLoop(cancel context.CancelFunc, cb SignalCallBack, l logr.Logger) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigs
	if sig == syscall.SIGINT { // ctrl-c
		cb.OnSigInt()
	} else {
		cb.OnSigTerm()
	}
}
