package main

import (
	"context"
	"flag"
	"os"
	"os/exec"
	"time"

	"github.com/golang/glog"

	"alb2/config"
	"alb2/controller"
)

func main() {
	flag.Parse()
	defer glog.Flush()
	glog.Error("Service start.")

	err := config.ValidateConfig()
	if err != nil {
		glog.Error(err.Error())
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	config.Set("LABEL_SERVICE_ID", "service.alauda.io/uuid")
	config.Set("LABEL_SERVICE_NAME", "service.alauda.io/name")
	config.Set("LABEL_CREATOR", "service.alauda.io/createby")
	go controller.RegisterLoop(ctx)

	if config.Get("LB_TYPE") == config.Haproxy ||
		config.Get("LB_TYPE") == config.Nginx {
		go rotateLog(ctx)
	}

	interval := config.GetInt("INTERVAL")
	for {
		glog.Flush()
		time.Sleep(time.Duration(interval) * time.Second)
		ch := make(chan string)

		go func() {
			err := controller.TryLockAlb()
			if err != nil {
				//lock is hold by another pod
				ch <- "continue"
				return
			}

			ctl, err := controller.GetController()
			if err != nil {
				glog.Error(err.Error())
				ch <- "continue"
				return
			}
			ch <- "wait"

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
		timer := time.NewTimer(300 * time.Second)

	watchdog:
		for {
			select {
			case msg := <-ch:
				if msg == "continue" {
					glog.Info("continue")
					timer.Stop()
					break watchdog
				}
				timer.Reset(300 * time.Second)
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
	glog.Info("rotateLog start")
	for {
		select {
		case <-ctx.Done():
			glog.Info("rotateLog exit")
			return
		case <-time.After(time.Minute):
			// Do nothin
		}
		output, err := exec.Command("/usr/sbin/logrotate", "/etc/logrotate.d/alauda").CombinedOutput()
		if err != nil {
			glog.Errorf("Rotate log failed %s %v", output, err)
		} else {
			glog.Info("Rotate log success")
		}
	}
}
