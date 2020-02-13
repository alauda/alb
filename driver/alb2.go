package driver

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"alb2/config"
	m "alb2/modules"
	alb2v1 "alb2/pkg/apis/alauda/v1"

	"github.com/evanphx/json-patch"
	"github.com/golang/glog"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	TypeAlb2     = "alaudaloadbalancer2"
	TypeFrontend = "frontends"
	TypeRule     = "rules"
)

func (kd *KubernetesDriver) LoadAlbResource(namespace, name string) (*alb2v1.ALB2, error) {
	alb, err := kd.ALBClient.CrdV1().ALB2s(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return alb, nil
}

func (kd *KubernetesDriver) UpdateAlbResource(alb *alb2v1.ALB2) error {
	newAlb, err := kd.ALBClient.CrdV1().ALB2s(alb.Namespace).Update(alb)
	if err != nil {
		glog.Errorf("Update alb %s.%s failed: %s", alb.Name, alb.Namespace, err.Error())
	}
	newAlb.Status = alb.Status
	_, err = kd.ALBClient.CrdV1().ALB2s(alb.Namespace).UpdateStatus(newAlb)
	if err != nil {
		glog.Errorf("Update alb status %s.%s failed: %s", alb.Name, alb.Namespace, err.Error())
	}
	return err
}

func UpdateSourceLabels(labels map[string]string, source *alb2v1.Source) {
	if source == nil {
		return
	}
	labels[fmt.Sprintf(config.Get("labels.source_type"), config.Get("DOMAIN"))] = source.Type
	labels[fmt.Sprintf(config.Get("labels.source_name"), config.Get("DOMAIN"))] = fmt.Sprintf("%s.%s", source.Name, source.Namespace)
}

// UpsertFrontends will create new frontend if it not exist, otherwise update
func (kd *KubernetesDriver) UpsertFrontends(alb *m.AlaudaLoadBalancer, ft *m.Frontend) error {
	glog.Infof("upsert frontend: %s", ft.Name)
	var ftRes *alb2v1.Frontend
	var err error
	ftRes, err = kd.ALBClient.CrdV1().Frontends(alb.Namespace).Get(ft.Name, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			ftRes = &alb2v1.Frontend{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: alb.Namespace,
					Name:      ft.Name,
					Labels:    map[string]string{},
					OwnerReferences: []metav1.OwnerReference{
						metav1.OwnerReference{
							APIVersion: alb2v1.SchemeGroupVersion.String(),
							Kind:       alb2v1.ALB2Kind,
							Name:       alb.Name,
							UID:        alb.UID,
						},
					},
				},
				Spec: ft.FrontendSpec,
			}
			ftRes.Labels[fmt.Sprintf(config.Get("labels.name"), config.Get("DOMAIN"))] = alb.Name
			UpdateSourceLabels(ftRes.Labels, ft.Source)
			ftRes, err = kd.ALBClient.CrdV1().Frontends(alb.Namespace).Create(ftRes)
			if err != nil {
				glog.Error(err)
				return err
			}
		} else {
			glog.Error(err)
			return err
		}
	}
	ftRes.Labels[fmt.Sprintf(config.Get("labels.name"), config.Get("DOMAIN"))] = alb.Name
	UpdateSourceLabels(ftRes.Labels, ft.Source)
	ftRes.Spec = ft.FrontendSpec
	_, err = kd.ALBClient.CrdV1().Frontends(alb.Namespace).Update(ftRes)
	if err != nil {
		glog.Error(err)
		return err
	}
	return nil
}

func (kd *KubernetesDriver) CreateRule(rule *m.Rule) error {
	ruleRes := &alb2v1.Rule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rule.Name,
			Namespace: rule.FT.LB.Namespace,
			Labels: map[string]string{
				fmt.Sprintf(config.Get("labels.name"), config.Get("DOMAIN")):     rule.FT.LB.Name,
				fmt.Sprintf(config.Get("labels.frontend"), config.Get("DOMAIN")): rule.FT.Name,
			},
			OwnerReferences: []metav1.OwnerReference{
				metav1.OwnerReference{
					APIVersion: alb2v1.SchemeGroupVersion.String(),
					Kind:       alb2v1.FrontendKind,
					Name:       rule.FT.Name,
					UID:        rule.FT.UID,
				},
			},
		},
		Spec: rule.RuleSpec,
	}
	UpdateSourceLabels(ruleRes.Labels, rule.Source)
	_, err := kd.ALBClient.CrdV1().Rules(ruleRes.Namespace).Create(ruleRes)
	if err != nil {
		glog.Error(err)
	}
	return err
}

func (kd *KubernetesDriver) DeleteRule(rule *m.Rule) error {
	err := kd.ALBClient.CrdV1().Rules(rule.FT.LB.Namespace).Delete(rule.Name, &metav1.DeleteOptions{})
	if err != nil {
		glog.Error(err)
	}
	return err
}

func (kd *KubernetesDriver) UpdateRule(rule *m.Rule) error {
	oldRule, err := kd.ALBClient.CrdV1().Rules(rule.FT.LB.Namespace).Get(rule.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	oldRule.Spec = rule.RuleSpec
	_, err = kd.ALBClient.CrdV1().Rules(rule.FT.LB.Namespace).Update(oldRule)
	if err != nil {
		return err
	}
	return nil
}

func (kd *KubernetesDriver) LoadFrontends(namespace, lbname string) ([]alb2v1.Frontend, error) {
	selector := fmt.Sprintf("%s=%s", fmt.Sprintf(config.Get("labels.name"), config.Get("DOMAIN")), lbname)
	resList, err := kd.ALBClient.CrdV1().Frontends(namespace).List(metav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		glog.Error(err)
		return nil, err
	}
	return resList.Items, nil
}

func (kd *KubernetesDriver) LoadRules(namespace, lbname, ftname string) ([]alb2v1.Rule, error) {
	selector := fmt.Sprintf(
		"%s=%s,%s=%s",
		fmt.Sprintf(config.Get("labels.name"), config.Get("DOMAIN")), lbname,
		fmt.Sprintf(config.Get("labels.frontend"), config.Get("DOMAIN")), ftname,
	)
	resList, err := kd.ALBClient.CrdV1().Rules(namespace).List(metav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		glog.Error(err)
		return nil, err
	}
	return resList.Items, nil
}

func (kd *KubernetesDriver) LoadALBbyName(namespace, name string) (*m.AlaudaLoadBalancer, error) {
	alb2 := m.AlaudaLoadBalancer{
		Name:      name,
		Namespace: namespace,
		Frontends: []*m.Frontend{},
	}
	alb2Res, err := kd.LoadAlbResource(namespace, name)
	if err != nil {
		glog.Error(err)
		return nil, err
	}
	alb2.UID = alb2Res.UID
	alb2.Spec = alb2Res.Spec
	cm, err := kd.LoadConfigmap(namespace, name)
	if err != nil {
		glog.Error(err)
		return nil, err
	}
	alb2.TweakHash = cm.ResourceVersion

	resList, err := kd.LoadFrontends(namespace, name)
	if err != nil {
		glog.Error(err)
		return nil, err
	}
	for _, res := range resList {
		ft := &m.Frontend{
			UID:          res.UID,
			Name:         res.Name,
			FrontendSpec: res.Spec,
			Rules:        []*m.Rule{},
			LB:           &alb2,
		}
		ruleList, err := kd.LoadRules(namespace, name, res.Name)
		if err != nil {
			glog.Error(err)
			return nil, err
		}

		for _, r := range ruleList {
			rule := &m.Rule{
				RuleSpec: r.Spec,
				Name:     r.Name,
				FT:       ft,
			}
			ft.Rules = append(ft.Rules, rule)
		}
		alb2.Frontends = append(alb2.Frontends, ft)
	}
	return &alb2, nil
}

func parseServiceGroup(data map[string]*Service, sg *alb2v1.ServiceGroup, allowNoAddr bool) (map[string]*Service, error) {
	if sg == nil {
		return data, nil
	}

	kd, err := GetDriver()
	if err != nil {
		glog.Error(err)
		return data, err
	}

	for _, svc := range sg.Services {
		key := svc.String()
		if _, ok := data[key]; !ok {
			service, err := kd.GetServiceByName(svc.Namespace, svc.Name, svc.Port)
			if err != nil {
				glog.Errorf("Get service address for %s.%s:%d failed:%s",
					svc.Namespace, svc.Name, svc.Port, err.Error(),
				)
				if !allowNoAddr {
					continue
				}
			} else {
				glog.V(4).Infof("Get serivce %+v", *service)
			}
			data[key] = service
		}
	}
	return data, nil
}

func LoadServices(alb *m.AlaudaLoadBalancer) ([]*Service, error) {
	var err error
	data := make(map[string]*Service)

	for _, ft := range alb.Frontends {
		data, err = parseServiceGroup(data, ft.ServiceGroup, ft.AllowNoAddr())
		if err != nil {
			glog.Error(err)
			return nil, err
		}

		for _, rule := range ft.Rules {
			data, err = parseServiceGroup(data, rule.ServiceGroup, rule.AllowNoAddr())
			if err != nil {
				glog.Error(err)
				return nil, err
			}
		}
	}

	services := make([]*Service, 0, len(data))
	for _, svc := range data {
		services = append(services, svc)
	}
	return services, nil
}

func (kd *KubernetesDriver) UpdateFrontendStatus(ftName string, conflict bool) error {
	ft, err := kd.ALBClient.CrdV1().Frontends(config.Get("NAMESPACE")).Get(ftName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	origin := ft.DeepCopy()
	hostname, err := os.Hostname()
	if err != nil {
		return err
	}
	if ft.Status.Instances == nil {
		ft.Status.Instances = make(map[string]alb2v1.Instance)
	}
	ft.Status.Instances[hostname] = alb2v1.Instance{
		Conflict:  conflict,
		ProbeTime: time.Now().Unix(),
	}
	bytesOrigin, err := json.Marshal(origin)
	if err != nil {
		return err
	}
	bytesModified, err := json.Marshal(ft)
	if err != nil {
		return err
	}
	patch, err := jsonpatch.CreateMergePatch(bytesOrigin, bytesModified)
	if err != nil {
		return err
	}
	if string(patch) == "{}" {
		return nil
	}
	if _, err := kd.ALBClient.CrdV1().Frontends(config.Get("NAMESPACE")).Patch(ft.Name, types.MergePatchType, patch, "status"); err != nil {
		return err
	}

	return nil
}

func (kd *KubernetesDriver) LoadConfigmap(namespace, lbname string) (*corev1.ConfigMap, error) {
	cm, err := kd.Client.CoreV1().ConfigMaps(namespace).Get(lbname, metav1.GetOptions{})
	if err != nil {
		glog.Error(err)
		return nil, err
	}
	return cm, nil
}
