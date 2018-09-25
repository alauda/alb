package driver

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/golang/glog"
	"github.com/parnurzeal/gorequest"

	"alauda_lb/config"
	m "alb2/modules"
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

type HttpClient interface {
	Do(typ, ns, name, selector string) (http.Response, string, error)
}

type defaultClient struct {
}

func (c *defaultClient) Do(typ, ns, name, selector string) (string, error) {
	var url string
	if name != "" {
		url = fmt.Sprintf("%s/apis/crd.alauda.io/v1/namespaces/%s/%s/%s",
			config.Get("KUBERNETES_SERVER"),
			ns, typ, name,
		)
	} else {
		url = fmt.Sprintf("%s/apis/crd.alauda.io/v1/namespaces/%s/%s",
			config.Get("KUBERNETES_SERVER"),
			ns, typ,
		)
	}
	client := GetK8sHTTPClient("GET", url)
	if selector != "" {
		client = client.Query(fmt.Sprintf("labelSelector=%s", selector))
	}
	resp, body, errs := client.End()
	if len(errs) > 0 {
		glog.Error(errs[0])
		return "", errs[0]
	}
	if resp.StatusCode != 200 {
		glog.Errorf("Request to %s get %d: %s", url, resp.StatusCode, body)
		return "", errors.New(body)
	}
	glog.Infof("Request to kubernetes %s success, get %d bytes.", resp.Request.URL, len(body))
	return body, nil
}

var client = &defaultClient{}

func LoadAlbResource(namespace, name string) (*m.Alb2Resource, error) {
	body, err := client.Do("alaudaloadbalancer2", namespace, name, "")
	if err != nil {
		glog.Error(err)
		return nil, err
	}
	var alb2Res m.Alb2Resource
	err = json.Unmarshal([]byte(body), &alb2Res)
	if err != nil {
		return nil, err
	}
	return &alb2Res, nil
}

func LoadFrontends(namespace, lbname string) ([]*m.FrontendResource, error) {
	selector := fmt.Sprintf("%s=%s", config.Get("labels.name"), lbname)
	body, err := client.Do("frontends", namespace, "", selector)
	if err != nil {
		glog.Error(err)
		return nil, err
	}

	var resList m.FrontendList
	err = json.Unmarshal([]byte(body), &resList)
	if err != nil {
		glog.Error(err)
		return nil, err
	}
	return resList.Items, nil
}

func LoadRules(namespace, lbname, ftname string) ([]*m.RuleResource, error) {
	selector := fmt.Sprintf(
		"%s=%s,%s=%s",
		config.Get("labels.name"), lbname,
		config.Get("labels.frontend"), ftname,
	)
	body, err := client.Do("rules", namespace, "", selector)
	if err != nil {
		glog.Error(err)
		return nil, err
	}

	var resList m.RuleList
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
	alb2.Alb2Spec = alb2Res.Spec

	resList, err := LoadFrontends(namespace, name)
	if err != nil {
		glog.Error(err)
		return nil, err
	}
	for _, res := range resList {
		ft := &m.Frontend{
			FrontendSpec: res.Spec,
			Rules:        []*m.Rule{},
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
			}
			ft.Rules = append(ft.Rules, rule)
		}
		alb2.Frontends = append(alb2.Frontends, ft)
	}
	return &alb2, nil
}
