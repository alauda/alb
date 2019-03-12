package controller

import (
	"alb2/config"
	"alb2/driver"
	m "alb2/modules"

	"github.com/golang/glog"
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
		for _, rl := range ft.Rules {
			if rl.Source != nil &&
				(rl.Source.Type == m.TypeBind || rl.Source.Type == m.TypeIngress) &&
				rl.ServiceGroup != nil && len(rl.ServiceGroup.Services) != 0 {
				noneExist := 0
				needDel := false
				for _, svc := range rl.ServiceGroup.Services {
					service, err := kd.Client.CoreV1().Services(svc.Namespace).Get(svc.Name, metav1.GetOptions{})
					if k8serrors.IsNotFound(err) {
						noneExist++
						continue
					}
					if rl.Source.Type == m.TypeBind {
						// handle service unbind lb in UI
						jsonInfo := service.Annotations[config.Get("labels.bindkey")]
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
	return nil
}
