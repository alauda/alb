package alb

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"net/http/pprof"

	"alauda.io/alb2/config"
	ctl "alauda.io/alb2/controller"
	"alauda.io/alb2/controller/modules"
	"alauda.io/alb2/controller/state"
	"alauda.io/alb2/driver"
	gateway "alauda.io/alb2/gateway"
	gctl "alauda.io/alb2/gateway/ctl"
	"alauda.io/alb2/ingress"
	"alauda.io/alb2/utils"
	"alauda.io/alb2/utils/log"
	"github.com/go-logr/logr"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

type Alb struct {
	ctx       context.Context
	cfg       *rest.Config
	albcfg    *config.Config
	lc        *ctl.LeaderElection
	portProbe *ctl.PortProbe
	log       logr.Logger
}

func NewAlb(ctx context.Context, restcfg *rest.Config, albCfg *config.Config, log logr.Logger) *Alb {
	return &Alb{
		ctx:    ctx,
		cfg:    restcfg,
		albcfg: albCfg,
		log:    log,
	}
}

func (a *Alb) WithLc(lc *ctl.LeaderElection) {
	a.lc = lc
}

func (a *Alb) Start() error {
	ctx := a.ctx
	lc := a.lc
	l := log.L().WithName("lifecycle")
	l.Info("ALB start.")

	albCfg := config.GetConfig()
	state.GetState().SetPhase(modules.PhaseStarting)
	leaderctx := a.lc.GetCtx()
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
	err = driver.InitDriver(drv, ctx)
	if err != nil {
		return err
	}

	if albCfg.Controller.Flags.EnablePortProbe {
		port, err := ctl.NewPortProbe(ctx, drv, l.WithName("portprobe"), albCfg)
		if err != nil {
			l.Error(err, "init portprobe fail")
		}
		a.portProbe = port
	}

	enableIngress := albCfg.EnableIngress()
	// start ingress loop
	l.Info("SERVE_INGRESS", "enable", enableIngress)
	if enableIngress {
		informers := drv.Informers
		ingressController := ingress.NewController(drv, informers, albCfg, log.L().WithName("ingress"))
		go func() {
			l.Info("ingress loop, start wait leader")
			a.lc.WaitUtilIMLeader()
			l.Info("ingress loop, im leader now")
			err := ingressController.StartIngressLoop(leaderctx)
			if err != nil {
				l.Error(err, "ingress loop fail", "ctx", leaderctx.Err())
			}
		}()
		go func() {
			l.Info("ingress-resync loop, start wait leader")
			a.lc.WaitUtilIMLeader()
			l.Info("ingress-resync loop, im leader now")
			err := ingressController.StartResyncLoop(leaderctx)
			if err != nil {
				l.Error(err, "ingress resync loop fail", "ctx", leaderctx.Err())
			}
		}()
	}

	// start gateway loop
	{
		gcfg := config.GetConfig().GetGatewayCfg()
		l := log.L().WithName(gateway.ALB_GATEWAY_CONTROLLER)
		enableGateway := gcfg.Enable
		l.Info("init gateway ", "enable", enableGateway)

		if enableGateway {
			go func() {
				l.Info("wait leader")
				lc.WaitUtilIMLeader()
				l.Info("im leader,start gateway controller")
				gctl.Run(leaderctx)
			}()
		}
	}

	flags := config.GetConfig().GetFlags()
	klog.Infof("reload nginx %v", flags.ReloadNginx)

	if flags.ReloadNginx {
		go a.StartReloadLoadBalancerLoop(drv, ctx)
	}
	if flags.EnableGoMonitor {
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
	interval := time.Duration(config.GetConfig().GetInterval()) * time.Second
	reloadTimeout := time.Duration(config.GetConfig().GetReloadTimeout()) * time.Second
	log := a.log
	klog.Infof("reload: interval is %v  reloadtimeout is %v", interval, reloadTimeout)

	isTimeout := utils.UtilWithContextAndTimeout(ctx, func() {
		startTime := time.Now()

		nctl := ctl.NewNginxController(drv, ctx, a.albcfg, log.WithName("nginx"), a.lc)
		nctl.PortProber = a.portProbe
		// do leader stuff
		if a.lc.AmILeader() {
			if a.portProbe != nil {
				err := a.portProbe.LeaderUpdateAlbPortStatus()
				if err != nil {
					log.Error(err, "leader update alb status fail")
				}
			}
			nctl.GC()
		}

		if config.GetConfig().GetFlags().DisablePeroidGenNginxConfig {
			klog.Infof("reload: period regenerated config disabled")
			return
		}
		err := nctl.GenerateConf()
		if err != nil {
			klog.Error(err.Error())
			return
		}
		err = nctl.ReloadLoadBalancer()
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

func (a *Alb) StartGoMonitorLoop(ctx context.Context) error {
	// TODO fixme
	// TODO how to stop it? use http server with ctx.
	log := a.log.WithName("monitor")
	flags := config.GetConfig().GetFlags()
	if !flags.EnableGoMonitor {
		log.Info("disable, ignore")
		return nil
	}
	port := config.GetConfig().GetGoMonitorPort()
	if port == 0 {
		port = 1937
		log.Info("not specific port find, use default", "port", port)
	}

	log.Info("init", "port", port)

	mux := http.NewServeMux()

	if flags.EnableProfile {
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
