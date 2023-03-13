package main

import (
	"alauda.io/alb2/alb"
	"alauda.io/alb2/config"
	"alauda.io/alb2/driver"
	"alauda.io/alb2/modules"
	"alauda.io/alb2/utils/log"
	"context"
	"github.com/go-logr/logr"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	defer log.Flush()
	err := run()
	if err != nil {
		log.L().Error(err, "run fail")
		log.Flush()
		os.Exit(-1)
	}
}

func run() error {
	err := config.Init()
	if err != nil {
		return err
	}
	l := log.L().WithName("lifecycle")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	restcfg, err := driver.GetKubeCfg()
	if err != nil {
		l.Error(err, "get rest cfg fail")
		return err
	}
	albCfg := config.GetConfig()
	if err != nil {
		l.Error(err, "get alb cfg fail")
		return err
	}

	go StartSignalLoop(cancel, SignalCallBack{
		OnSigInt: func() {
			l.Info("receive SIGINT(ctrl-c), shutting down")
			cancel()
			os.Exit(0)
		},
		OnSigTerm: func() {
			l.Info("receive SIGTERM(graceful-shutdown), close nginx port")
			config.Set("PHASE", modules.PhaseTerminating)
			//  could not cancel here. waitting f5 healthcheck to remove this port.
			//  wait nginx close 1936 metrics port
			//  then we could stop alb.
		},
	}, log.L().WithName("signal"))

	a := alb.NewAlb(ctx, restcfg, albCfg, log.L())
	err = a.Start()
	if err != nil {
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
