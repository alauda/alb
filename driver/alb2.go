package driver

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"alauda.io/alb2/config"
	m "alauda.io/alb2/modules"
	alb2v1 "alauda.io/alb2/pkg/apis/alauda/v1"
	"alauda.io/alb2/utils/dirhash"
	jsonpatch "github.com/evanphx/json-patch"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
)

const (
	TypeAlb2     = "alaudaloadbalancer2"
	TypeFrontend = "frontends"
	TypeRule     = "rules"
)

func (kd *KubernetesDriver) LoadAlbResource(namespace, name string) (*alb2v1.ALB2, error) {
	alb, err := kd.ALB2Lister.ALB2s(namespace).Get(name)
	if err != nil {
		return nil, err
	}
	return alb, nil
}

func (kd *KubernetesDriver) UpdateAlbResource(alb *alb2v1.ALB2) error {
	newAlb, err := kd.ALBClient.CrdV1().ALB2s(alb.Namespace).Update(context.TODO(), alb, metav1.UpdateOptions{})
	if err != nil {
		klog.Errorf("Update alb %s.%s failed: %s", alb.Name, alb.Namespace, err.Error())
		return err
	}
	newAlb.Status = alb.Status
	_, err = kd.ALBClient.CrdV1().ALB2s(alb.Namespace).UpdateStatus(context.TODO(), newAlb, metav1.UpdateOptions{})
	if err != nil {
		klog.Errorf("Update alb status %s.%s failed: %s", alb.Name, alb.Namespace, err.Error())
		return err
	}
	return nil
}

func UpdateSourceLabels(labels map[string]string, source *alb2v1.Source) {
	if source == nil {
		return
	}
	labels[config.GetLabelSourceType()] = source.Type
	labels[config.GetLabelSourceIngressHash()] = HashSource(source)
}

func HashSource(source *alb2v1.Source) string {
	return dirhash.LabelSafetHash(fmt.Sprintf("%s.%s", source.Name, source.Namespace))
}

// UpsertFrontends will create new frontend if it not exist, otherwise update
func (kd *KubernetesDriver) UpsertFrontends(alb *m.AlaudaLoadBalancer, ft *m.Frontend) error {
	klog.Infof("upsert frontend: %s", ft.Name)
	var ftRes *alb2v1.Frontend
	var err error
	ftRes, err = kd.FrontendLister.Frontends(alb.Namespace).Get(ft.Name)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			ftRes = &alb2v1.Frontend{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: alb.Namespace,
					Name:      ft.Name,
					Labels:    map[string]string{},
					OwnerReferences: []metav1.OwnerReference{
						{
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

			ftRes, err = kd.ALBClient.CrdV1().Frontends(alb.Namespace).Create(context.TODO(), ftRes, metav1.CreateOptions{})
			if err != nil {
				klog.Error(err)
				return err
			}
		} else {
			klog.Error(err)
			return err
		}
	}
	ftRes.Labels[fmt.Sprintf(config.Get("labels.name"), config.Get("DOMAIN"))] = alb.Name
	ftRes.Spec = ft.FrontendSpec
	ftRes, err = kd.ALBClient.CrdV1().Frontends(alb.Namespace).Update(context.TODO(), ftRes, metav1.UpdateOptions{})
	if err != nil {
		klog.Error(err)
		return err
	}
	ft.UID = ftRes.UID
	return nil
}

func (kd *KubernetesDriver) CreateRule(rule *m.Rule) error {
	ruleRes := &alb2v1.Rule{
		ObjectMeta: metav1.ObjectMeta{
			Name:        rule.Name,
			Namespace:   rule.FT.LB.Namespace,
			Annotations: rule.Annotations,
			Labels: map[string]string{
				fmt.Sprintf(config.Get("labels.name"), config.Get("DOMAIN")):     rule.FT.LB.Name,
				fmt.Sprintf(config.Get("labels.frontend"), config.Get("DOMAIN")): rule.FT.Name,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
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
	_, err := kd.ALBClient.CrdV1().Rules(ruleRes.Namespace).Create(context.TODO(), ruleRes, metav1.CreateOptions{})
	if err != nil {
		klog.Error(err)
	}
	return err
}

func (kd *KubernetesDriver) DeleteRule(rule *m.Rule) error {
	err := kd.ALBClient.CrdV1().Rules(rule.FT.LB.Namespace).Delete(context.TODO(), rule.Name, metav1.DeleteOptions{})
	if err != nil {
		klog.Error(err)
	}
	return err
}

func (kd *KubernetesDriver) UpdateRule(rule *m.Rule) error {
	oldRule, err := kd.RuleLister.Rules(rule.FT.LB.Namespace).Get(rule.Name)
	if err != nil {
		return err
	}

	oldRule.Spec = rule.RuleSpec
	_, err = kd.ALBClient.CrdV1().Rules(rule.FT.LB.Namespace).Update(context.TODO(), oldRule, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}

func (kd *KubernetesDriver) LoadFrontends(namespace, lbname string) ([]*alb2v1.Frontend, error) {
	sel := labels.Set{fmt.Sprintf(config.Get("labels.name"), config.Get("DOMAIN")): lbname}.AsSelector()
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
		fmt.Sprintf(config.Get("labels.name"), config.Get("DOMAIN")):     lbname,
		fmt.Sprintf(config.Get("labels.frontend"), config.Get("DOMAIN")): ftname,
	}.AsSelector()
	resList, err := kd.RuleLister.Rules(namespace).List(sel)
	if err != nil {
		klog.Error(err)
		return nil, err
	}
	return resList, nil
}

func (kd *KubernetesDriver) LoadALBbyName(namespace, name string) (*m.AlaudaLoadBalancer, error) {
	alb2 := m.AlaudaLoadBalancer{
		Name:      name,
		Namespace: namespace,
		Frontends: []*m.Frontend{},
	}
	alb2Res, err := kd.LoadAlbResource(namespace, name)
	klog.V(4).Infof("load alb key %s/%s: uid %v", namespace, name, alb2Res.UID)
	if err != nil {
		klog.Error(err)
		return nil, err
	}
	alb2.UID = alb2Res.UID
	alb2.Spec = alb2Res.Spec
	alb2.Labels = alb2Res.Labels

	// calculate hash by tweak dir
	hash, err := dirhash.HashDir(config.Get("TWEAK_DIRECTORY"), ".conf", dirhash.DefaultHash)
	if err != nil {
		klog.Error(err)
		return nil, err
	}
	alb2.TweakHash = hash

	resList, err := kd.LoadFrontends(namespace, name)
	if err != nil {
		klog.Error(err)
		return nil, err
	}
	for _, res := range resList {
		ft := &m.Frontend{
			UID:          res.UID,
			Name:         res.Name,
			Lables:       res.Labels,
			FrontendSpec: res.Spec,
			Rules:        []*m.Rule{},
			LB:           &alb2,
		}
		ruleList, err := kd.LoadRules(namespace, name, res.Name)
		if err != nil {
			klog.Error(err)
			return nil, err
		}

		for _, r := range ruleList {
			rule := &m.Rule{
				Annotations: r.Annotations,
				RuleSpec:    r.Spec,
				Name:        r.Name,
				Labels:      r.Labels,
				FT:          ft,
			}
			ft.Rules = append(ft.Rules, rule)
		}
		alb2.Frontends = append(alb2.Frontends, ft)
	}
	return &alb2, nil
}

func (kd *KubernetesDriver) UpdateFrontendStatus(ftName string, conflictState bool) error {
	ft, err := kd.FrontendLister.Frontends(config.Get("NAMESPACE")).Get(ftName)
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

	preConflictState := false
	if instance, ok := ft.Status.Instances[hostname]; ok {
		preConflictState = instance.Conflict
	}

	if preConflictState == conflictState {
		return nil
	}

	ft.Status.Instances[hostname] = alb2v1.Instance{
		Conflict:  conflictState,
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
	if _, err := kd.ALBClient.CrdV1().Frontends(config.Get("NAMESPACE")).Patch(context.TODO(), ft.Name, types.MergePatchType, patch, metav1.PatchOptions{}, "status"); err != nil {
		return err
	}

	return nil
}

func (kd *KubernetesDriver) LoadConfigmap(namespace, lbname string) (*corev1.ConfigMap, error) {
	cm, err := kd.Client.CoreV1().ConfigMaps(namespace).Get(context.TODO(), lbname, metav1.GetOptions{})
	if err != nil {
		klog.Error(err)
		return nil, err
	}
	return cm, nil
}
