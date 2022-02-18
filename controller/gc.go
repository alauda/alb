package controller

import (
	"context"
	"fmt"

	"alauda.io/alb2/config"
	"alauda.io/alb2/driver"
	m "alauda.io/alb2/modules"
	alb2v1 "alauda.io/alb2/pkg/apis/alauda/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

type GCOptions struct {
	GCAppRule     bool
	GCServiceRule bool
}

type GCReason int

const (
	FTServiceNonExist GCReason = iota
	FTServiceBindkeyEmpty
	RuleOrphaned
	RuleAllSerivceNonExist
	RuleSerivceBindkeyEmpty
)

func (r GCReason) String() string {
	switch r {
	case FTServiceNonExist:
		return "frontend default serivce not fould"
	case FTServiceBindkeyEmpty:
		return "frontend default serivce bindkey is empty"
	case RuleOrphaned:
		return "rule is orphaned"
	case RuleAllSerivceNonExist:
		return "rule all serivce non exist"
	case RuleSerivceBindkeyEmpty:
		return "rule one of service bindkey is empty"
	default:
		return fmt.Sprintf("undefined reason: %d", int(r))
	}

}

type GCActionKind int

const (
	UpdateFT GCActionKind = iota
	DeleteRule
)

type GCAction struct {
	Kind      GCActionKind
	Reason    GCReason
	Namespace string
	Name      string // frontend name when kind is update-frontend, rule name when kind is delete-rule
}

func calculateGCActions(kd *driver.KubernetesDriver, opt GCOptions) (actions []GCAction, err error) {
	namespace := config.Get("NAMESPACE")
	alb, err := kd.LoadALBbyName(namespace, config.Get("NAME"))
	if err != nil {
		klog.Error(err)
		return nil, err
	}

	gcActions := []GCAction{}

	if opt.GCAppRule {
		for _, ft := range alb.Frontends {
			if ft.IsHttpOrHttps() {
				for _, rl := range ft.Rules {
					orphaned, err := kd.RuleIsOrphanedByApplication(rl)
					if err != nil {
						klog.Warningf("verify if the rule %s is orphaned error: %v, ignored", rl.Name, err)
						continue
					}
					if orphaned {
						gcActions = append(gcActions, GCAction{
							Namespace: namespace,
							Kind:      DeleteRule,
							Name:      rl.Name,
							Reason:    RuleOrphaned,
						})
					}
				}
			}
		}
	}

	if opt.GCServiceRule {
		for _, ft := range alb.Frontends {
			if ft.IsTcpOrUdp() {
				// gc frontend
				hasDefaultBackendService := ft.Source != nil && ft.Source.Type == m.TypeBind && ft.ServiceGroup != nil && len(ft.ServiceGroup.Services) == 1
				if hasDefaultBackendService {
					svc := ft.ServiceGroup.Services[0]
					service, err := kd.ServiceLister.Services(svc.Namespace).Get(svc.Name)
					if err != nil {
						if k8serrors.IsNotFound(err) {
							gcActions = append(gcActions, GCAction{
								Namespace: namespace,
								Kind:      UpdateFT,
								Name:      ft.Name,
								Reason:    FTServiceNonExist,
							})
						}
					} else {
						bindkey := service.Annotations[fmt.Sprintf(config.Get("labels.bindkey"), config.Get("DOMAIN"))]
						if bindkey == "" || bindkey == "[]" {
							gcActions = append(gcActions, GCAction{
								Namespace: namespace,
								Kind:      UpdateFT,
								Name:      ft.Name,
								Reason:    FTServiceBindkeyEmpty,
							})
						}
					}
				}
			}

			if ft.IsHttpOrHttps() {
				// gc rules
				for _, rl := range ft.Rules {
					if rl.RedirectURL != "" {
						// for redirect rule, service is meaningless
						continue
					}
					if rl.Source != nil && rl.Source.Type == m.TypeIngress {
						// only gc bind rules
						continue
					}
					// gc bind rules
					isValidBindRule := rl.Source != nil &&
						rl.Source.Type == m.TypeBind &&
						rl.ServiceGroup != nil && len(rl.ServiceGroup.Services) != 0

					if isValidBindRule {
						noneExist := 0
						bindkeyEmpty := false
						for _, svc := range rl.ServiceGroup.Services {
							service, err := kd.ServiceLister.Services(svc.Namespace).Get(svc.Name)
							if err != nil && k8serrors.IsNotFound(err) {
								noneExist++
								continue
							}
							if err == nil {
								// handle service unbind lb in UI
								bindkey := service.Annotations[fmt.Sprintf(config.Get("labels.bindkey"), config.Get("DOMAIN"))]
								if bindkey == "" || bindkey == "[]" {
									klog.Warningf("service bind key is empty ns:%s serivce:%s \n", svc.Namespace, svc.Name)
									bindkeyEmpty = true
									break
								}
							}
						}
						if bindkeyEmpty {
							gcActions = append(gcActions, GCAction{
								Namespace: namespace,
								Kind:      DeleteRule,
								Name:      rl.Name,
								Reason:    RuleSerivceBindkeyEmpty,
							})
						}
						if noneExist == len(rl.ServiceGroup.Services) {
							gcActions = append(gcActions, GCAction{
								Namespace: namespace,
								Kind:      DeleteRule,
								Name:      rl.Name,
								Reason:    RuleAllSerivceNonExist,
							})
						}
					}
				}
			}
		}
	}
	return gcActions, nil
}

func GCRule(kd *driver.KubernetesDriver, opt GCOptions) error {
	gcActions, err := calculateGCActions(kd, opt)
	if err != nil {
		return err
	}

	for _, action := range gcActions {
		if action.Kind == UpdateFT {
			klog.Infof("gc update-frontend ns:%s name:%s reason:%s", action.Namespace, action.Name, action.Reason.String())
			ftRes, err := kd.FrontendLister.Frontends(action.Namespace).Get(action.Name)
			if err != nil {
				klog.Error(err)
				continue
			}
			ftRes.Spec.ServiceGroup.Services = []alb2v1.Service{}
			if _, err := kd.ALBClient.CrdV1().Frontends(config.Get("NAMESPACE")).Update(context.TODO(), ftRes, metav1.UpdateOptions{}); err != nil {
				klog.Error(err)
			}
		}

		if action.Kind == DeleteRule {
			klog.Infof("gc delete-rule ns:%s name:%s reason:%s", action.Namespace, action.Name, action.Reason.String())
			err := kd.ALBClient.CrdV1().Rules(action.Namespace).Delete(context.TODO(), action.Name, metav1.DeleteOptions{})
			if err != nil {
				klog.Error(err)
			}
		}
	}
	return nil
}
