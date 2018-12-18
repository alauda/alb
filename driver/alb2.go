package driver

import (
	"fmt"
	"time"

	"github.com/golang/glog"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"

	"alb2/config"
	m "alb2/modules"
	alb2v1 "alb2/pkg/apis/alauda/v1"
	albclient "alb2/pkg/client/clientset/versioned"
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
	_, err := kd.ALBClient.CrdV1().ALB2s(alb.Namespace).Update(alb)
	if err != nil {
		glog.Errorf("Update alb %s.%s failed: %s", alb.Name, alb.Namespace, err.Error())
	}
	return err
}

func UpdateSourceLabels(labels map[string]string, source *alb2v1.Source) {
	if source == nil {
		return
	}
	labels[config.Get("labels.source_type")] = source.Type
	labels[config.Get("labels.source_name")] = fmt.Sprintf("%s.%s", source.Name, source.Namespace)
}

// UpsertFrontends will create new frontend if it not exist, otherwise update
func (kd *KubernetesDriver) UpsertFrontends(alb *m.AlaudaLoadBalancer, ft *m.Frontend) error {
	ftRes, err := kd.ALBClient.CrdV1().Frontends(alb.Namespace).Get(ft.Name, metav1.GetOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		glog.Error(err)
		return err
	}
	ftRes.Labels[config.Get("labels.name")] = alb.Name
	UpdateSourceLabels(ftRes.Labels, ft.Source)
	ftRes.Spec = ft.FrontendSpec

	if err != nil {
		_, err = kd.ALBClient.CrdV1().Frontends(alb.Namespace).Create(ftRes)
	} else {
		_, err = kd.ALBClient.CrdV1().Frontends(alb.Namespace).Update(ftRes)
	}
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
				config.Get("labels.name"):     rule.FT.LB.Name,
				config.Get("labels.frontend"): rule.FT.Name,
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

func (kd *KubernetesDriver) LoadFrontends(namespace, lbname string) ([]alb2v1.Frontend, error) {
	selector := fmt.Sprintf("%s=%s", config.Get("labels.name"), lbname)
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
		config.Get("labels.name"), lbname,
		config.Get("labels.frontend"), ftname,
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
	alb2.Spec = alb2Res.Spec

	resList, err := kd.LoadFrontends(namespace, name)
	if err != nil {
		glog.Error(err)
		return nil, err
	}
	for _, res := range resList {
		ft := &m.Frontend{
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

func parseServiceGroup(data map[string]*Service, sg *alb2v1.ServiceGroup) (map[string]*Service, error) {
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
				glog.Infof("Get service address for %s.%s:%d failed:%s",
					svc.Namespace, svc.Name, svc.Port, err.Error(),
				)
				continue
			}
			glog.Infof("Get serivce %+v", *service)
			data[key] = service
		}
	}
	return data, nil
}

func LoadServices(alb *m.AlaudaLoadBalancer) ([]*Service, error) {
	var err error
	data := make(map[string]*Service)

	for _, ft := range alb.Frontends {
		data, err = parseServiceGroup(data, ft.ServiceGroup)
		if err != nil {
			glog.Error(err)
			return nil, err
		}

		for _, rule := range ft.Rules {
			data, err = parseServiceGroup(data, rule.ServiceGroup)
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

func GetALBClient() (*albclient.Clientset, error) {
	conf, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	conf.Timeout = time.Second * time.Duration(config.GetInt("KUBERNETES_TIMEOUT"))
	conf.Insecure = true
	albClient, err := albclient.NewForConfig(conf)
	if err != nil {
		return nil, err
	}
	return albClient, nil
}
