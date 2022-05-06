package alb

import (
	"context"
	"fmt"
	"net/http"
	"time"

	_ "net/http/pprof"

	"alauda.io/alb2/config"
	ctl "alauda.io/alb2/controller"
	"alauda.io/alb2/driver"
	gateway "alauda.io/alb2/gateway"
	gctl "alauda.io/alb2/gateway/ctl"
	"alauda.io/alb2/ingress"
	"alauda.io/alb2/modules"
	"alauda.io/alb2/utils"
	"alauda.io/alb2/utils/log"
	"k8s.io/klog/v2"
)

func Init() {
	log.Init()
}

// start alb, block until ctx is done
func Start(ctx context.Context) {
	defer klog.Flush()
	klog.Info("lifecycle: ALB start.")
	config.Set("PHASE", modules.PhaseStarting)
	if config.GetBool("PPROF") {
		go func() {
			port := config.GetInt("PPROF_PORT")
			if port == 0 {
				port = 8080
			}
			klog.Infof("lifecycle: start pprof web %d", port)
			err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
			if err != nil {
				klog.Error(err.Error())
			}
		}()
	}

	err := config.ValidateConfig()
	if err != nil {
		klog.Error(err.Error())
		return
	}

	drv, err := driver.GetDriver()
	if err != nil {
		panic(err)
	}
	driver.InitDriver(drv, ctx)

	informers := drv.Informers

	klog.Info("SERVE_INGRESS:", config.GetBool("SERVE_INGRESS"))
	if config.GetBool("SERVE_INGRESS") {
		ingressController := ingress.NewController(drv, informers.Alb.Alb, informers.Alb.Rule, informers.K8s.Ingress, informers.K8s.IngressClass, informers.K8s.Namespace.Lister())
		go ingressController.Start(ctx)
	}

	if config.Get("ENABLE_PROFILE") == "true" {
		go func() {
			// for profiling
			http.ListenAndServe(":1937", nil)
		}()
	}

	{
		l := log.L().WithName(gateway.ALB_GATEWAY_CONTROLLER)
		enableGateway := config.GetBool("ENABLE_GATEWAY")
		l.Info("init gateway ", "enable", enableGateway)

		if enableGateway {
			go func() {
				l.Info("wait leader")
				ctl.WaitUtilIMLeader(ctx, drv)
				l.Info("im leader,start gateway controller")
				gctl.Run(ctx)
			}()
		}
	}

	klog.Infof("reload nginx %v", config.GetBool("RELOAD_NGINX"))

	if config.GetBool("RELOAD_NGINX") {
		go reloadLoadBalancer(drv, ctx)
	}
	<-ctx.Done()

	klog.Infof("lifecycle: ctx is done")
}

// start reload alb, block util ctx is done
// reload in every INTERVAL sec
// it will gc rules, generate nginx config and reload nginx, assume that those cr really take effect.
// TODO add a work queue.
func reloadLoadBalancer(drv *driver.KubernetesDriver, ctx context.Context) {
	interval := time.Duration(config.GetInt("INTERVAL")) * time.Second
	reloadTimeout := time.Duration(config.GetInt("RELOAD_TIMEOUT")) * time.Second
	klog.Infof("reload: interval is %v  reloadtimeout is %v", interval, reloadTimeout)

	isTimeout := utils.UtilWithContextAndTimeout(ctx, func() {
		startTime := time.Now()
		if err := ctl.TryLockAlbAndUpdateAlbStatus(drv); err != nil {
			klog.Error(err.Error())
		}

		ctl, err := ctl.GetController(drv)
		if err != nil {
			klog.Error(err.Error())
			return
		}
		// TODO should only leader do GC?
		ctl.GC()
		if config.GetBool("DISABLE_PEROID_GEN_NGINX_CONFIG") {
			klog.Infof("reload: period regenerated config disabled")
			return
		}
		err = ctl.GenerateConf()
		if err != nil {
			klog.Error(err.Error())
			return
		}
		err = ctl.ReloadLoadBalancer()
		if err != nil {
			klog.Error(err.Error())
		}
		klog.Infof("reload: End update reload loop, cost %s", time.Since(startTime))
	}, reloadTimeout, interval)

	// just crash if timeout
	if isTimeout {
		klog.Error("reload timeout")
		klog.Flush()
		panic("reach the timeout %v %v")
	}

	klog.Infof("reload: ctx is done")
}
