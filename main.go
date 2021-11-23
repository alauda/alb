package main

import (
	"alauda.io/alb2/alb"
	"alauda.io/alb2/config"
	"alauda.io/alb2/modules"
	"context"
	"k8s.io/klog"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	alb.Init()
	sigs := make(chan os.Signal, 1)
	ctx, cancel := context.WithCancel(context.Background())
	// register a signal notifier for SIGTERM
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigs
		if sig == syscall.SIGINT {
			klog.Info("receive SIGINT, shutting down")
			cancel() // make go vet happy
			os.Exit(0)
		} else {
			klog.Info("receive SIGTERM preparing for terminating")
			config.Set("PHASE", modules.PhaseTerminating)
		}
	}()
	alb.Start(ctx)
}
