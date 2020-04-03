package main

import (
	"alauda.io/alb2/driver"
	albinformers "alauda.io/alb2/pkg/client/informers/externalversions"
	"context"
	"flag"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/exec"
	"time"

	"k8s.io/klog"

	"alauda.io/alb2/config"
	"alauda.io/alb2/controller"
	"alauda.io/alb2/ingress"
)

func main() {
	klog.InitFlags(nil)
	flag.Parse()
	defer klog.Flush()
	klog.Info("ALB start.")

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
	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(drv.Client, time.Second*180)
	ingressInformer := kubeInformerFactory.Networking().V1beta1().Ingresses()
	ingressSynced := ingressInformer.Informer().HasSynced
	serviceInformer := kubeInformerFactory.Core().V1().Services()
	serviceLister := serviceInformer.Lister()
	serviceSynced := serviceInformer.Informer().HasSynced
	endpointInformer := kubeInformerFactory.Core().V1().Endpoints()
	endpointLister := endpointInformer.Lister()
	endpointSynced := endpointInformer.Informer().HasSynced
	kubeInformerFactory.Start(ctx.Done())

	albInformerFactory := albinformers.NewSharedInformerFactoryWithOptions(drv.ALBClient, 0,
		albinformers.WithNamespace(config.Get("NAMESPACE")))
	alb2Informer := albInformerFactory.Crd().V1().ALB2s()
	alb2Lister := alb2Informer.Lister()
	alb2Synced := alb2Informer.Informer().HasSynced
	frontendInformer := albInformerFactory.Crd().V1().Frontends()
	frontendLister := frontendInformer.Lister()
	frontendSynced := frontendInformer.Informer().HasSynced
	ruleInformer := albInformerFactory.Crd().V1().Rules()
	ruleLister := ruleInformer.Lister()
	ruleSynced := ruleInformer.Informer().HasSynced
	albInformerFactory.Start(ctx.Done())
	drv.FillUpListers(serviceLister, endpointLister, alb2Lister, frontendLister, ruleLister)

	klog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(ctx.Done(), ingressSynced, serviceSynced, endpointSynced, alb2Synced, frontendSynced, ruleSynced); !ok {
		klog.Fatalf("failed to wait for caches to sync")
	}

	ingressController := ingress.NewController(drv, ingressInformer)
	go ingressController.Start(ctx)
	go func() {
		// for profiling
		http.ListenAndServe(":1937", nil)
	}()

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
