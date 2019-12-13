package controller

import (
	"alb2/config"
	"alb2/driver"
	m "alb2/modules"
	"fmt"

	alb2v1 "alb2/pkg/apis/alauda/v1"

	"github.com/golang/glog"
	"github.com/thoas/go-funk"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GCRule(kd *driver.KubernetesDriver) error {
	alb, err := kd.LoadALBbyName(config.Get("NAMESPACE"), config.Get("NAME"))
	if err != nil {
		glog.Error(err)
		return err
	}
	for _, ft := range alb.Frontends {
		if funk.ContainsString([]string{m.ProtoTCP, m.ProtoUDP}, ft.Protocol) {
			// protocol tcp or udp has no rules
			if ft.Source != nil && ft.Source.Type == m.TypeBind && ft.ServiceGroup != nil && len(ft.ServiceGroup.Services) == 1 {
				svc := ft.ServiceGroup.Services[0]
				service, err := kd.Client.CoreV1().Services(svc.Namespace).Get(svc.Name, metav1.GetOptions{})
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
					ftRes, err := kd.ALBClient.CrdV1().Frontends(config.Get("NAMESPACE")).Get(ft.Name, metav1.GetOptions{})
					if err != nil {
						glog.Error(err)
						continue
					}
					ftRes.Spec.ServiceGroup.Services = []alb2v1.Service{}
					if _, err := kd.ALBClient.CrdV1().Frontends(config.Get("NAMESPACE")).Update(ftRes); err != nil {
						glog.Error(err)
					}
				}
			}
		} else {
			for _, rl := range ft.Rules {
				if rl.Source != nil &&
					(rl.Source.Type == m.TypeBind || rl.Source.Type == m.TypeIngress) &&
					rl.ServiceGroup != nil && len(rl.ServiceGroup.Services) != 0 {
					noneExist := 0
					needDel := false
					for _, svc := range rl.ServiceGroup.Services {
						service, err := kd.Client.CoreV1().Services(svc.Namespace).Get(svc.Name, metav1.GetOptions{})
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
						glog.Infof("delete rule %s in gc", rl.Name)
						err := kd.ALBClient.CrdV1().Rules(config.Get("NAMESPACE")).Delete(rl.Name, &metav1.DeleteOptions{})
						if err != nil {
							glog.Error(err)
						}
					}
				}
			}
		}
	}
	return nil
}
