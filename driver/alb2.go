package driver

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"

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

func LoadAlbResource(namespace, name string) (*m.Alb2Resource, error) {
	url := fmt.Sprintf("%s/apis/crd.alauda.io/v1/namespaces/%s/alaudaloadbalancer2/%s",
		config.Get("KUBERNETES_SERVER"),
		namespace,
		name,
	)
	resp, body, errs := GetK8sHTTPClient("GET", url).End()
	if len(errs) > 0 {
		glog.Error(errs[0])
		return nil, errs[0]
	}
	if resp.StatusCode != 200 {
		glog.Errorf("Request to %s get %d: %s", url, resp.StatusCode, body)
		return nil, errors.New(body)
	}
	glog.Infof("Request to kubernetes %s success, get %d bytes.", resp.Request.URL, len(body))
	var alb2Res m.Alb2Resource
	err := json.Unmarshal([]byte(body), &alb2Res)
	if err != nil {
		return nil, err
	}
	return &alb2Res, nil
}

func LoadALBbyName(namespace, name string) (*m.AlaudaLoadBalancer, error) {
	alb2 := m.AlaudaLoadBalancer{
		Name:      name,
		Namespace: namespace,
	}
	alb2Res, err := LoadAlbResource(namespace, name)
	if err != nil {
		glog.Error(err)
		return nil, err
	}
	alb2.Alb2Spec = alb2Res.Spec
	return &alb2, nil
}
