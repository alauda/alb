package alb

import (
	"context"
	"fmt"
	"net/http"
	"net/http/pprof"
	"time"

	"alauda.io/alb2/config"
	ctl "alauda.io/alb2/controller"
	"alauda.io/alb2/controller/modules"
	"alauda.io/alb2/controller/state"
	"alauda.io/alb2/driver"
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
	albCfg    *config.Config
	le        *ctl.LeaderElection
	portProbe *ctl.PortProbe
	log       logr.Logger
}

func NewAlb(ctx context.Context, restCfg *rest.Config, albCfg *config.Config, le *ctl.LeaderElection, log logr.Logger) *Alb {
	return &Alb{
		ctx:    ctx,
		cfg:    restCfg,
		albCfg: albCfg,
		le:     le,
		log:    log,
	}
}

func (a *Alb) Start() error {
	l := log.L().WithName("lifecycle")
	l.Info("ALB start.")

	state.GetState().SetPhase(modules.PhaseStarting)

	go func() {
		l.Info("start leaderelection")
		err := a.le.StartLeaderElectionLoop()
		if err != nil {
			l.Error(err, "leader election fail")
		}
	}()

	drv, err := driver.GetAndInitDriver(a.ctx)
	if err != nil {
		return err
	}

	if a.albCfg.Controller.Flags.EnablePortProbe {
		port, err := ctl.NewPortProbe(a.ctx, drv, l.WithName("portprobe"), a.albCfg)
		if err != nil {
			l.Error(err, "init portprobe fail")
		}
		a.portProbe = port
	}

	leaderCtx := a.le.GetCtx()
	enableIngress := a.albCfg.EnableIngress()
	// start ingress loop
	l.Info("SERVE_INGRESS", "enable", enableIngress)
	if enableIngress {
		informers := drv.Informers
		ingressController := ingress.NewController(drv, informers, a.albCfg, log.L().WithName("ingress"))
		go func() {
			l.Info("ingress loop, start wait leader")
			a.le.WaitUtilIMLeader()
			l.Info("ingress loop, im leader now")
			err := ingressController.StartIngressLoop(leaderCtx)
			if err != nil {
				l.Error(err, "ingress loop fail", "ctx", leaderCtx.Err())
			}
		}()
		go func() {
			l.Info("ingress-resync loop, start wait leader")
			a.le.WaitUtilIMLeader()
			l.Info("ingress-resync loop, im leader now")
			err := ingressController.StartResyncLoop(leaderCtx)
			if err != nil {
				l.Error(err, "ingress resync loop fail", "ctx", leaderCtx.Err())
			}
		}()
	}

	// start gateway loop
	l.Info("init gateway ", "enable", a.albCfg.Gateway.Enable)
	if a.albCfg.Gateway.Enable {
		go func() {
			l.Info("wait leader")
			a.le.WaitUtilIMLeader()
			l.Info("im leader,start gateway controller")
			gctl.Run(leaderCtx)
		}()
	}

	flags := a.albCfg.GetFlags()
	klog.Infof("reload nginx %v", flags.ReloadNginx)

	if flags.ReloadNginx {
		go a.StartReloadLoadBalancerLoop(drv, a.ctx)
	}
	if flags.EnableGoMonitor {
		go a.StartGoMonitorLoop(a.ctx)
	}

	// wait util ctx is cancel(signal)
	<-a.ctx.Done()
	klog.Infof("lifecycle: ctx is done")
	return nil
}

// start reload alb, block util ctx is done
// reload in every INTERVAL sec
// it will gc rules, generate nginx config and reload nginx, assume that those cr really take effect.
// TODO add a work queue.
func (a *Alb) StartReloadLoadBalancerLoop(drv *driver.KubernetesDriver, ctx context.Context) {
	interval := time.Duration(a.albCfg.GetInterval()) * time.Second
	reloadTimeout := time.Duration(a.albCfg.GetReloadTimeout()) * time.Second
	log := a.log
	klog.Infof("reload: interval is %v  reload timeout is %v", interval, reloadTimeout)

	isTimeout := utils.UtilWithContextAndTimeout(ctx, func() {
		startTime := time.Now()

		nctl := ctl.NewNginxController(drv, ctx, a.albCfg, log.WithName("nginx"), a.le)
		nctl.PortProber = a.portProbe
		// do leader stuff
		if a.le.AmILeader() {
			if a.portProbe != nil {
				err := a.portProbe.LeaderUpdateAlbPortStatus()
				if err != nil {
					log.Error(err, "leader update alb status fail")
				}
			}
			nctl.GC()
		}

		if a.albCfg.GetFlags().DisablePeriodGenNginxConfig {
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

func (a *Alb) StartGoMonitorLoop(ctx context.Context) {
	// TODO fixme
	// TODO how to stop it? use http server with ctx.
	log := a.log.WithName("monitor")
	flags := a.albCfg.GetFlags()
	if !flags.EnableGoMonitor {
		log.Info("disable, ignore")
	}
	port := a.albCfg.GetGoMonitorPort()
	if port == 0 {
		port = 1937
		log.Info("no specific port found, use default", "port", port)
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
	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), mux); err != nil {
		log.Error(err, "init metrics http handle fail")
	}
}
