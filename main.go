package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/exec"
	"time"

	"github.com/golang/glog"

	"alb2/config"
	"alb2/controller"
	"alb2/driver"
	"alb2/ingress"
	"alb2/utils"
)

func main() {
	flag.Set("alsologtostderr", "true")
	flag.Parse()
	utils.InitLog()
	defer glog.Flush()
	glog.Error("Service start.")

	err := config.ValidateConfig()
	if err != nil {
		glog.Error(err.Error())
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	config.Set("LABEL_SERVICE_ID", fmt.Sprintf("service.%s/uuid", config.Get("DOMAIN")))
	config.Set("LABEL_SERVICE_NAME", fmt.Sprintf("service.%s/name", config.Get("DOMAIN")))
	config.Set("LABEL_CREATOR", fmt.Sprintf("service.%s/createby", config.Get("DOMAIN")))

	d, err := driver.GetDriver()
	if err != nil {
		panic(err)
	}
	// install necessary crd on start
	if config.GetBool("INSTALL_CRD") {
		if err := d.RegisterCustomDefinedResources(); err != nil {
			// install crd failed, abort
			panic(err)
		}
	}

	go ingress.MainLoop(ctx)
	go func() {
		// for profiling
		http.ListenAndServe(":1937", nil)
	}()

	if config.Get("LB_TYPE") == config.Nginx {
		go rotateLog(ctx)
	}

	interval := config.GetInt("INTERVAL")
	tmo := time.Duration(config.GetInt("RELOAD_TIMEOUT")) * time.Second
	for {
		time.Sleep(time.Duration(interval) * time.Second)
		ch := make(chan string)

		go func() {
			err := controller.TryLockAlb()
			if err != nil {
				glog.Error("lock alb failed", err.Error())
			}
			ctl, err := controller.GetController()
			if err != nil {
				glog.Error(err.Error())
				ch <- "continue"
				return
			}
			ch <- "wait"

			ctl.GC()
			err = ctl.GenerateConf()
			if err != nil {
				glog.Error(err.Error())
				ch <- "continue"
				return
			}
			ch <- "wait"
			err = ctl.ReloadLoadBalancer()
			if err != nil {
				glog.Error(err.Error())
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
					glog.Info("continue")
					timer.Reset(0)
					break watchdog
				}
				timer.Reset(tmo)
				continue
			case <-timer.C:
				glog.Error("reload timeout")
				glog.Flush()
				os.Exit(1)
			}
		}

	}
}

func rotateLog(ctx context.Context) {
	rotateInterval := config.GetInt("ROTATE_INTERVAL")
	glog.Info("rotateLog start, rotate interval ", rotateInterval)
	for {
		select {
		case <-ctx.Done():
			glog.Info("rotateLog exit")
			return
		case <-time.After(time.Duration(rotateInterval) * time.Minute):
			err := utils.RotateGlog(time.Now().Add(-time.Duration(rotateInterval) * time.Minute))
			if err != nil {
				glog.Errorf("rotate glog failed, %+v", err)
			}
			// Do nothin
		}
		glog.Info("start rorate log")
		output, err := exec.Command("/usr/sbin/logrotate", "/etc/logrotate.d/alauda").CombinedOutput()
		if err != nil {
			glog.Errorf("rotate log failed %s %v", output, err)
		} else {
			glog.Info("rotate log success")
		}
	}
}
