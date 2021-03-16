package controller

import (
	"alauda.io/alb2/config"
	"alauda.io/alb2/driver"
	m "alauda.io/alb2/modules"
	alb2v1 "alauda.io/alb2/pkg/apis/alauda/v1"
	"fmt"

	"github.com/thoas/go-funk"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
)

type GCOptions struct {
	GCAppRule     bool
	GCServiceRule bool
}

func GCRule(kd *driver.KubernetesDriver, opt GCOptions) error {
	alb, err := kd.LoadALBbyName(config.Get("NAMESPACE"), config.Get("NAME"))
	if err != nil {
		klog.Error(err)
		return err
	}
	for _, ft := range alb.Frontends {
		if funk.ContainsString([]string{m.ProtoTCP, m.ProtoUDP}, ft.Protocol) {
			if !opt.GCServiceRule {
				continue
			}
			// protocol tcp or udp has no rules
			if ft.Source != nil && ft.Source.Type == m.TypeBind && ft.ServiceGroup != nil && len(ft.ServiceGroup.Services) == 1 {
				svc := ft.ServiceGroup.Services[0]
				service, err := kd.ServiceLister.Services(svc.Namespace).Get(svc.Name)
				needDel := false
				if err != nil {
					if k8serrors.IsNotFound(err) {
						needDel = true
					}
				} else {
					jsonInfo := service.Annotations[fmt.Sprintf(config.Get("labels.bindkey"), config.Get("DOMAIN"))]
					if jsonInfo == "" || jsonInfo == "[]" {
						needDel = true
					}
				}
				if needDel {
					ftRes, err := kd.FrontendLister.Frontends(config.Get("NAMESPACE")).Get(ft.Name)
					if err != nil {
						klog.Error(err)
						continue
					}
					ftRes.Spec.ServiceGroup.Services = []alb2v1.Service{}
					if _, err := kd.ALBClient.CrdV1().Frontends(config.Get("NAMESPACE")).Update(ftRes); err != nil {
						klog.Error(err)
					}
				}
			}
		} else {
			for _, rl := range ft.Rules {
				if opt.GCAppRule {
					orphaned, err := kd.RuleIsOrphanedByApplication(rl)
					if err != nil {
						klog.Warningf("verify if the rule %s is orphaned error: %v, ignored", rl.Name, err)
					} else if orphaned {
						// Delete the orphaned rule
						klog.Infof("Delete the orphaned application rule %s in gc", rl.Name)
						err := kd.ALBClient.CrdV1().Rules(config.Get("NAMESPACE")).Delete(rl.Name, &metav1.DeleteOptions{})
						if err != nil {
							klog.Error(err)
							continue
						}
					}
				}

				if !opt.GCServiceRule {
					continue
				}
				if rl.RedirectURL != "" {
					// for redirect rule, service is meaningless
					continue
				}
				if rl.Source != nil &&
					(rl.Source.Type == m.TypeBind || rl.Source.Type == m.TypeIngress) &&
					rl.ServiceGroup != nil && len(rl.ServiceGroup.Services) != 0 {
					noneExist := 0
					needDel := false
					for _, svc := range rl.ServiceGroup.Services {
						service, err := kd.ServiceLister.Services(svc.Namespace).Get(svc.Name)
						if err != nil {
							if k8serrors.IsNotFound(err) {
								noneExist++
							}
							continue
						}
						if rl.Source.Type == m.TypeBind {
							// handle service unbind lb in UI
							jsonInfo := service.Annotations[fmt.Sprintf(config.Get("labels.bindkey"), config.Get("DOMAIN"))]
							if jsonInfo == "" || jsonInfo == "[]" {
								needDel = true
								break
							}
						}
					}
					if noneExist == len(rl.ServiceGroup.Services) || needDel {
						// all services associate with rule are not exist any more
						klog.Infof("delete rule %s in gc", rl.Name)
						err := kd.ALBClient.CrdV1().Rules(config.Get("NAMESPACE")).Delete(rl.Name, &metav1.DeleteOptions{})
						if err != nil {
							klog.Error(err)
						}
					}
				}
			}
		}
	}
	return nil
}
