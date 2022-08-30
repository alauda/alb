package alb

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"net/http/pprof"

	"alauda.io/alb2/config"
	ctl "alauda.io/alb2/controller"
	"alauda.io/alb2/driver"
	gateway "alauda.io/alb2/gateway"
	gctl "alauda.io/alb2/gateway/ctl"
	"alauda.io/alb2/ingress"
	"alauda.io/alb2/metrics"
	"alauda.io/alb2/modules"
	"alauda.io/alb2/utils"
	"alauda.io/alb2/utils/log"
	"github.com/go-logr/logr"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

type Alb struct {
	ctx    context.Context
	cfg    *rest.Config
	albcfg config.IConfig
	lc     *LeaderElection
	log    logr.Logger
}

func NewAlb(ctx context.Context, restcfg *rest.Config, albCfg config.IConfig, log logr.Logger) *Alb {
	lc := NewLeaderElection(ctx, albCfg, restcfg, log.WithName("lc"))
	return &Alb{
		ctx:    ctx,
		cfg:    restcfg,
		albcfg: albCfg,
		log:    log,
		lc:     lc,
	}
}

func (a *Alb) Start() error {
	ctx := a.ctx
	lc := a.lc
	l := log.L().WithName("lifecycle")
	l.Info("ALB start.")

	albCfg := config.GetConfig()
	config.Set("PHASE", modules.PhaseStarting)

	go func() {
		l.Info("start leaderelection")
		err := a.lc.StartLeaderElectionLoop()
		if err != nil {
			l.Error(err, "leader election fail")
		}
	}()

	drv, err := driver.GetDriver(ctx)
	if err != nil {
		return err
	}
	driver.InitDriver(drv, ctx)

	// start ingress loop
	l.Info("SERVE_INGRESS", "enable", config.GetBool("SERVE_INGRESS"))
	if config.GetBool("SERVE_INGRESS") {
		informers := drv.Informers
		ingressController := ingress.NewController(drv, informers, albCfg, log.L().WithName("ingress"))
		go func() {
			l.Info("ingress loop, start wait leader")
			a.lc.WaitUtilIMLeader()
			l.Info("ingress loop, im leader now")
			err := ingressController.StartIngressLoop(ctx)
			if err != nil {
				l.Error(err, "ingress loop fail")
			}
		}()
		go func() {
			l.Info("ingress-resync loop, start wait leader")
			a.lc.WaitUtilIMLeader()
			l.Info("ingress-resync loop, im leader now")
			err := ingressController.StartResyncLoop(ctx)
			if err != nil {
				l.Error(err, "ingress resync loop fail")
			}
		}()
	}

	// start gateway loop
	{
		l := log.L().WithName(gateway.ALB_GATEWAY_CONTROLLER)
		enableGateway := config.GetBool("ENABLE_GATEWAY")
		l.Info("init gateway ", "enable", enableGateway)

		if enableGateway {
			go func() {
				l.Info("wait leader")
				lc.WaitUtilIMLeader()
				l.Info("im leader,start gateway controller")
				gctl.Run(ctx)
			}()
		}
	}

	klog.Infof("reload nginx %v", config.GetBool("RELOAD_NGINX"))

	if config.GetBool("RELOAD_NGINX") {
		go a.StartReloadLoadBalancerLoop(drv, ctx)
	}
	if config.GetBool("ENABLE_GO_MONITOR") {
		go a.StartGoMonitorLoop(ctx)
	}

	// wait util ctx is cancel(signal)
	<-ctx.Done()
	klog.Infof("lifecycle: ctx is done")
	return nil
}

// start reload alb, block util ctx is done
// reload in every INTERVAL sec
// it will gc rules, generate nginx config and reload nginx, assume that those cr really take effect.
// TODO add a work queue.
func (a *Alb) StartReloadLoadBalancerLoop(drv *driver.KubernetesDriver, ctx context.Context) {
	interval := time.Duration(config.GetInt("INTERVAL")) * time.Second
	reloadTimeout := time.Duration(config.GetInt("RELOAD_TIMEOUT")) * time.Second
	log := a.log
	klog.Infof("reload: interval is %v  reloadtimeout is %v", interval, reloadTimeout)

	isTimeout := utils.UtilWithContextAndTimeout(ctx, func() {
		startTime := time.Now()

		ctl := ctl.NewNginxController(drv, ctx)
		// do leader stuff
		if a.lc.AmILeader() {
			err := LeaderUpdateAlbStatus(drv, a.albcfg)
			if err != nil {
				log.Error(err, "leader update alb status fail")
			}
			ctl.GC()
		}

		if config.GetBool("DISABLE_PEROID_GEN_NGINX_CONFIG") {
			klog.Infof("reload: period regenerated config disabled")
			return
		}
		err := ctl.GenerateConf()
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

	// TODO did we ready need this?
	// just crash if timeout
	if isTimeout {
		klog.Error("reload timeout")
		klog.Flush()
		// TODO release leader..
		panic(fmt.Sprintf("reach the timeout %v", reloadTimeout))
	}

	klog.Infof("reload: end of reload loop")
}

// report ft status
// TODO where should i place this function?
// TODO 这东西和生成nginx配置有关系吗? 是否一定要放在一起?
func LeaderUpdateAlbStatus(kd *driver.KubernetesDriver, cfg config.IConfig) error {
	name := cfg.GetAlbName()
	namespace := cfg.GetNs()
	albRes, err := kd.LoadAlbResource(namespace, name)
	if err != nil {
		klog.Errorf("Get alb %s.%s failed: %s", name, namespace, err.Error())
		return err
	}
	if albRes.Annotations == nil {
		albRes.Annotations = make(map[string]string)
	}

	// leader election use lease object now, this annotation just for identify who is leader.
	albRes.Annotations[cfg.GetLabelLeader()] = cfg.GetPodName()
	fts, err := kd.LoadFrontends(namespace, name)
	if err != nil {
		return err
	}
	state := "ready"
	reason := ""
	for _, ft := range fts {
		if ft.Status.Instances != nil {
			for _, v := range ft.Status.Instances {
				if v.Conflict {
					state = "warning"
					reason = "port conflict" // TODO 这是显示在界面上的报错吗? 如果是的,那最好把 pod port 也写出来.
					break
				}
			}
		}
	}
	albRes.Status.State = state
	albRes.Status.Reason = reason
	albRes.Status.ProbeTime = time.Now().Unix()
	err = kd.UpdateAlbResource(albRes)
	return err
}

func (a *Alb) StartGoMonitorLoop(ctx context.Context) error {
	// TODO how to stop it? use http server with ctx.
	log := a.log.WithName("monitor")
	if !config.GetBool("ENABLE_GO_MONITOR") {
		log.Info("disable, ignore")
		return nil
	}
	port := config.GetInt("GO_MONITOR_PORT")
	if port == 0 {
		port = 1937
		log.Info("not specific port find, use default", "port", port)
	}

	log.Info("init", "port", port)

	mux := http.NewServeMux()
	mux.Handle("/metrics", metrics.Handler())

	if config.GetBool("ENABLE_PROFILE") {
		mux.HandleFunc("/debug/pprof/", pprof.Index)
		mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	}
	err := http.ListenAndServe(fmt.Sprintf(":%d", port), mux)
	if err != nil {
		log.Error(err, "init metrics http handle fail")
		return err
	}

	return nil
}
