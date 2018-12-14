package driver

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	u "net/url"
	"time"

	"github.com/golang/glog"
	"github.com/parnurzeal/gorequest"
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

func GetK8sHTTPClient(method, url string) *gorequest.SuperAgent {
	client := gorequest.New().
		TLSClientConfig(&tls.Config{InsecureSkipVerify: true}).
		Type("json").
		Set("Authorization", fmt.Sprintf("Bearer %s", config.Get("KUBERNETES_BEARERTOKEN"))).
		Set("Accept", "application/json")
	client.Method = method
	client.Url = url
	return client
}

func GetUrl(typ, ns, name string) string {
	var url string
	url = fmt.Sprintf("%s/apis/crd.alauda.io/v1/namespaces/%s/%s",
		config.Get("KUBERNETES_SERVER"),
		ns, typ,
	)
	if name != "" {
		url = fmt.Sprintf("%s/%s", url, name)
	}
	return url
}

type HttpClient interface {
	Get(typ, ns, name, selector string) (data string, err error)
	Create(typ, ns, name string, resource interface{}) error
	Update(typ, ns, name string, resource interface{}) error
	Delete(typ, ns, name string) error
}

type defaultClient struct {
}

var ErrNotFount = errors.New("resource not found")

func (c *defaultClient) Get(typ, ns, name, selector string) (string, error) {
	url := GetUrl(typ, ns, name)
	client := GetK8sHTTPClient("GET", url)
	if selector != "" {
		query := u.QueryEscape(selector)
		client = client.Query(fmt.Sprintf("labelSelector=%s", query))
	}
	resp, body, errs := client.End()
	if len(errs) > 0 {
		glog.Error(errs[0])
		return "", errs[0]
	}
	if resp.StatusCode != 200 {
		glog.Errorf("Get %s with query %+v get %d: %s", client.Url, client.QueryData, resp.StatusCode, body)
		if resp.StatusCode == 404 {
			return "", ErrNotFount
		}
		return "", errors.New(body)
	}
	glog.Infof("Request to kubernetes %s success, get %d bytes.", resp.Request.URL, len(body))
	return body, nil
}

func (c *defaultClient) Create(typ, ns, name string, resource interface{}) error {
	url := GetUrl(typ, ns, "")
	client := GetK8sHTTPClient("POST", url)
	data, err := json.Marshal(resource)
	if err != nil {
		glog.Error(err)
		return err
	}
	resp, body, errs := client.Send(string(data)).End()
	if len(errs) > 0 {
		glog.Error(errs[0])
		return errs[0]
	}
	// POST will return 201
	if resp.StatusCode >= 400 {
		glog.Errorf("POST %s get %d: %s", url, resp.StatusCode, body)
		return errors.New(body)
	}
	return nil
}

func (c *defaultClient) Update(typ, ns, name string, resource interface{}) error {
	url := GetUrl(typ, ns, name)
	client := GetK8sHTTPClient("PUT", url)
	data, err := json.Marshal(resource)
	if err != nil {
		glog.Error(err)
		return err
	}

	resp, body, errs := client.Send(string(data)).End()
	if len(errs) > 0 {
		glog.Error(errs[0])
		return errs[0]
	}

	if resp.StatusCode >= 400 {
		glog.Errorf("Request to %s get %d: %s", url, resp.StatusCode, body)
		return errors.New(body)
	}
	return nil
}

func (c *defaultClient) Delete(typ, ns, name string) error {
	url := GetUrl(typ, ns, name)
	client := GetK8sHTTPClient("DELETE", url)
	resp, body, errs := client.End()
	if len(errs) > 0 {
		glog.Error(errs[0])
		return errs[0]
	}

	if resp.StatusCode >= 400 {
		glog.Errorf("Request to %s get %d: %s", url, resp.StatusCode, body)
		return errors.New(body)
	}
	return nil
}

var client HttpClient = &defaultClient{}

func LoadAlbResource(namespace, name string) (*alb2v1.ALB2, error) {
	body, err := client.Get(TypeAlb2, namespace, name, "")
	if err != nil {
		glog.Error(err)
		return nil, err
	}
	var alb2Res alb2v1.ALB2
	err = json.Unmarshal([]byte(body), &alb2Res)
	if err != nil {
		return nil, err
	}
	return &alb2Res, nil
}

func UpdateAlbResource(alb *alb2v1.ALB2) error {
	err := client.Update(TypeAlb2, alb.Namespace, alb.Name, alb)
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
func UpsertFrontends(alb *m.AlaudaLoadBalancer, ft *m.Frontend) error {
	ftdata, err := client.Get(TypeFrontend, alb.Namespace, ft.Name, "")
	if err != nil && err != ErrNotFount {
		glog.Error(err)
		return err
	}
	ftRes := alb2v1.Frontend{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Frontend",
			APIVersion: "crd.alauda.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: alb.Namespace,
			Name:      ft.Name,
			Labels:    map[string]string{},
		},
	}
	if len(ftdata) > 0 {
		err := json.Unmarshal([]byte(ftdata), &ftRes)
		if err != nil {
			glog.Error(err)
			return err
		}
	}

	ftRes.Labels[config.Get("labels.name")] = alb.Name
	UpdateSourceLabels(ftRes.Labels, ft.Source)
	ftRes.Spec = ft.FrontendSpec

	if err != nil {
		err = client.Create(TypeFrontend, alb.Namespace, ft.Name, ftRes)
	} else {
		err = client.Update(TypeFrontend, alb.Namespace, ft.Name, ftRes)
	}
	if err != nil {
		glog.Error(err)
		return err
	}
	return nil
}

func CreateRule(rule *m.Rule) error {
	ruleRes := alb2v1.Rule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rule.Name,
			Namespace: rule.FT.LB.Namespace,
			Labels: map[string]string{
				config.Get("labels.name"):     rule.FT.LB.Name,
				config.Get("labels.frontend"): rule.FT.Name,
			},
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "Rule",
			APIVersion: "crd.alauda.io/v1",
		},
		Spec: rule.RuleSpec,
	}
	UpdateSourceLabels(ruleRes.Labels, rule.Source)
	err := client.Create(TypeRule, ruleRes.Namespace, ruleRes.Name, ruleRes)
	if err != nil {
		glog.Error(err)
	}
	return err
}

func DeleteRule(rule *m.Rule) error {
	err := client.Delete(TypeRule, rule.FT.LB.Namespace, rule.Name)
	if err != nil {
		glog.Error(err)
	}
	return err
}

func LoadFrontends(namespace, lbname string) ([]alb2v1.Frontend, error) {
	selector := fmt.Sprintf("%s=%s", config.Get("labels.name"), lbname)
	body, err := client.Get(TypeFrontend, namespace, "", selector)
	if err != nil {
		glog.Error(err)
		return nil, err
	}

	var resList alb2v1.FrontendList
	err = json.Unmarshal([]byte(body), &resList)
	if err != nil {
		glog.Error(err)
		return nil, err
	}
	return resList.Items, nil
}

func LoadRules(namespace, lbname, ftname string) ([]alb2v1.Rule, error) {
	selector := fmt.Sprintf(
		"%s=%s,%s=%s",
		config.Get("labels.name"), lbname,
		config.Get("labels.frontend"), ftname,
	)
	body, err := client.Get(TypeRule, namespace, "", selector)
	if err != nil {
		glog.Error(err)
		return nil, err
	}

	var resList alb2v1.RuleList
	err = json.Unmarshal([]byte(body), &resList)
	if err != nil {
		glog.Error(err)
		return nil, err
	}
	return resList.Items, nil
}

func LoadALBbyName(namespace, name string) (*m.AlaudaLoadBalancer, error) {
	alb2 := m.AlaudaLoadBalancer{
		Name:      name,
		Namespace: namespace,
		Frontends: []*m.Frontend{},
	}
	alb2Res, err := LoadAlbResource(namespace, name)
	if err != nil {
		glog.Error(err)
		return nil, err
	}
	alb2.Spec = alb2Res.Spec

	resList, err := LoadFrontends(namespace, name)
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
		ruleList, err := LoadRules(namespace, name, res.Name)
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
	conf := &rest.Config{
		Host:        config.Get("KUBERNETES_SERVER"),
		BearerToken: config.Get("KUBERNETES_BEARERTOKEN"),
		Timeout:     time.Second * time.Duration(config.GetInt("KUBERNETES_TIMEOUT")),
	}
	conf.Insecure = true
	albClient, err := albclient.NewForConfig(conf)
	if err != nil {
		return nil, err
	}
	return albClient, nil
}
