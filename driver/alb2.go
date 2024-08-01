package driver

import (
	"context"
	"fmt"

	m "alauda.io/alb2/controller/modules"
	alb2v1 "alauda.io/alb2/pkg/apis/alauda/v1"
	albv2 "alauda.io/alb2/pkg/apis/alauda/v2beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	TypeAlb2     = "alaudaloadbalancer2"
	TypeFrontend = "frontends"
	TypeRule     = "rules"
)

func (kd *KubernetesDriver) LoadAlbResource(namespace, name string) (*albv2.ALB2, error) {
	alb, err := kd.ALB2Lister.ALB2s(namespace).Get(name)
	if err != nil {
		return nil, fmt.Errorf("get alb fail err  %v | %v | %v", namespace, name, err)
	}
	return alb, nil
}

func (kd *KubernetesDriver) UpdateAlbStatus(alb *albv2.ALB2) error {
	_, err := kd.ALBClient.CrdV2beta1().ALB2s(alb.Namespace).UpdateStatus(context.TODO(), alb, metav1.UpdateOptions{})
	if err != nil {
		klog.Errorf("Update alb status %s.%s failed: %s", alb.Name, alb.Namespace, err.Error())
		return err
	}
	return nil
}

func (kd *KubernetesDriver) LoadFrontends(namespace, lbname string) ([]*alb2v1.Frontend, error) {
	sel := labels.Set{kd.n.GetLabelAlbName(): lbname}.AsSelector()
	resList, err := kd.FrontendLister.Frontends(namespace).List(sel)
	klog.V(4).Infof("loadft alb %s/%s: sel %v len %v", namespace, lbname, sel, len(resList))
	if err != nil {
		klog.Error(err)
		return nil, err
	}
	return resList, nil
}

func (kd *KubernetesDriver) LoadRules(namespace, lbname, ftname string) ([]*alb2v1.Rule, error) {
	sel := labels.Set{
		kd.n.GetLabelAlbName(): lbname,
		kd.n.GetLabelFt():      ftname,
	}.AsSelector()
	resList, err := kd.RuleLister.Rules(namespace).List(sel)
	if err != nil {
		klog.Error(err)
		return nil, err
	}
	return resList, nil
}

func (kd *KubernetesDriver) LoadALB(key client.ObjectKey) (*m.AlaudaLoadBalancer, error) {
	return kd.LoadALBbyName(key.Namespace, key.Name)
}

func (kd *KubernetesDriver) LoadALBbyName(namespace, name string) (*m.AlaudaLoadBalancer, error) {
	alb2 := m.AlaudaLoadBalancer{
		Name:      name,
		Namespace: namespace,
		Frontends: []*m.Frontend{},
	}
	alb2Res, err := kd.LoadAlbResource(namespace, name)
	if err != nil {
		klog.Error(err)
		return nil, err
	}
	klog.V(4).Infof("load alb key %s/%s: uid %v", namespace, name, alb2Res.UID)
	alb2.Alb = alb2Res
	alb2.Status = alb2Res.Status
	alb2.Spec = alb2Res.Spec
	alb2.Labels = alb2Res.Labels

	resList, err := kd.LoadFrontends(namespace, name)
	if err != nil {
		klog.Error(err)
		return nil, err
	}
	for _, res := range resList {
		if res == nil {
			continue
		}
		ft := &m.Frontend{
			Frontend: res,
			Rules:    []*m.Rule{},
			LB:       &alb2,
		}
		ruleList, err := kd.LoadRules(namespace, name, res.Name)
		if err != nil {
			klog.Error(err)
			return nil, err
		}

		for _, r := range ruleList {
			rule := &m.Rule{
				Rule: r,
				FT:   ft,
			}
			ft.Rules = append(ft.Rules, rule)
		}
		alb2.Frontends = append(alb2.Frontends, ft)
	}
	return &alb2, nil
}

func (kd *KubernetesDriver) LoadConfigmap(namespace, lbname string) (*corev1.ConfigMap, error) {
	cm, err := kd.Client.CoreV1().ConfigMaps(namespace).Get(context.TODO(), lbname, metav1.GetOptions{})
	if err != nil {
		klog.Error(err)
		return nil, err
	}
	return cm, nil
}
