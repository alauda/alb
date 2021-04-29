package main

import (
	"context"
	"flag"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"alauda.io/alb2/config"
	"alauda.io/alb2/controller"
	"alauda.io/alb2/driver"
	"alauda.io/alb2/ingress"
	"alauda.io/alb2/modules"
	"k8s.io/klog"
)

func main() {
	klog.InitFlags(nil)
	flag.Parse()
	defer klog.Flush()
	klog.Info("ALB start.")
	config.Set("PHASE", modules.PhaseStarting)

	sigs := make(chan os.Signal, 1)
	// register a signal notifier for SIGTERM
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigs
		if sig == syscall.SIGINT {
			klog.Info("receive SIGINT, shutting down")
			os.Exit(0)
		} else {
			klog.Info("receive SIGTERM preparing for terminating")
			config.Set("PHASE", modules.PhaseTerminating)
		}
	}()

	err := config.ValidateConfig()
	if err != nil {
		klog.Error(err.Error())
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	drv, err := driver.GetDriver()
	if err != nil {
		panic(err)
	}
	informers, _ := driver.InitInformers(drv, ctx, driver.InitInformersOptions{ErrorIfWaitSyncFail: false})
	drv.FillUpListers(
		informers.K8s.Service.Lister(),
		informers.K8s.Endpoint.Lister(),
		informers.Alb.Alb.Lister(),
		informers.Alb.Ft.Lister(),
		informers.Alb.Rule.Lister())

	klog.Info("SERVE_INGRESS:", config.GetBool("SERVE_INGRESS"))
	if config.GetBool("SERVE_INGRESS") {
		ingressController := ingress.NewController(drv, informers.Alb.Alb, informers.Alb.Rule, informers.K8s.Ingress, informers.K8s.Namespace.Lister())
		go ingressController.Start(ctx)
	}
	if config.Get("ENABLE_PROFILE") == "true" {
		go func() {
			// for profiling
			http.ListenAndServe(":1937", nil)
		}()
	}

	if config.Get("LB_TYPE") == config.Nginx {
		go rotateLog()
	}

	interval := config.GetInt("INTERVAL")
	tmo := time.Duration(config.GetInt("RELOAD_TIMEOUT")) * time.Second
	for {
		time.Sleep(time.Duration(interval) * time.Second)
		ch := make(chan string)
		startTime := time.Now()
		klog.Info("Begin update reload loop")

		go func() {
			err := controller.TryLockAlb(drv)
			if err != nil {
				klog.Error("lock alb failed", err.Error())
			}
			ctl, err := controller.GetController(drv)
			if err != nil {
				klog.Error(err.Error())
				ch <- "continue"
				return
			}
			ch <- "wait"

			ctl.GC()
			err = ctl.GenerateConf()
			if err != nil {
				klog.Error(err.Error())
				ch <- "continue"
				return
			}
			ch <- "wait"
			err = ctl.ReloadLoadBalancer()
			if err != nil {
				klog.Error(err.Error())
			}
			ch <- "continue"
			return
		}()
		timer := time.NewTimer(tmo)

	watchdog:
		for {
			select {
			case msg := <-ch:
				if msg == "continue" {
					klog.Info("continue")
					timer.Reset(0)
					break watchdog
				}
				timer.Reset(tmo)
				continue
			case <-timer.C:
				klog.Error("reload timeout")
				klog.Flush()
				os.Exit(1)
			}
		}

		klog.Infof("End update reload loop, cost %s", time.Since(startTime))
	}
}

func rotateLog() {
	rotateInterval := config.GetInt("ROTATE_INTERVAL")
	klog.Info("rotateLog start, rotate interval ", rotateInterval)
	for {
		klog.Info("start rorate log")
		output, err := exec.Command("/usr/sbin/logrotate", "/etc/logrotate.d/alauda").CombinedOutput()
		if err != nil {
			klog.Errorf("rotate log failed %s %v", output, err)
		} else {
			klog.Info("rotate log success")
		}
		time.Sleep(time.Duration(rotateInterval) * time.Minute)
	}
}
