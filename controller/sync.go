package controller

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/golang/glog"
	"github.com/parnurzeal/gorequest"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"alauda_lb/config"
)

type ALB struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	Spec              *LoadBalancer `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
}

type ALBList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// List of services
	Items []ALB `json:"items" protobuf:"bytes,2,rep,name=items"`
}

type ALBCrd struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	Spec              json.RawMessage `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
}

type LBPair struct {
	alb *ALB
	lb  *LoadBalancer
}

func (p *LBPair) String() string {
	return fmt.Sprintf("[lb=%v, alb=%v]", p.lb != nil, p.alb != nil)
}

func printFuncLog(name string, start time.Time, err error) {
	dur := float64(time.Now().Sub(start)) / float64(time.Millisecond)
	glog.Infof("%s cost %.3fms.", name, dur)
	if err != nil {
		glog.Errorf("%s failed: %s", name, err.Error())
	} else {
		glog.Infof("%s successed.", name)
	}
}

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

func FetchALBInfo() (albs []ALB, err error) {
	defer printFuncLog("FetchALBInfo", time.Now(), err)
	url := fmt.Sprintf("%s/apis/alauda.io/v3/alaudaloadbalancers", config.Get("KUBERNETES_SERVER"))
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
	var albList ALBList
	err = json.Unmarshal([]byte(body), &albList)
	if err != nil {
		return nil, err
	}
	return albList.Items, nil
}

func FetchCRD() (crd *ALBCrd, err error) {
	defer printFuncLog("FetchCRD", time.Now(), err)
	url := fmt.Sprintf("%s/apis/apiextensions.k8s.io/v1beta1/customresourcedefinitions/alaudaloadbalancers.alauda.io", config.Get("KUBERNETES_SERVER"))
	resp, body, errs := GetK8sHTTPClient("GET", url).End()
	if len(errs) > 0 {
		glog.Error(errs[0])
		return nil, errs[0]
	}
	if resp.StatusCode != 200 {
		glog.Errorf("Request to %s get %d: %s", url, resp.StatusCode, body)
		return nil, errors.New(body)
	}
	glog.Infof("Request to kubernetes %s success, get %s.", resp.Request.URL, body)
	// make sure crd exist
	return nil, nil
}

func loadCRD(ctx context.Context) *ALBCrd {
	for {
		_, err := FetchCRD()
		if err == nil {
			glog.Info("Fetch ALB CRD success.")
			return nil
		}
		select {
		case <-ctx.Done():
			glog.Infof("loadCRD exit because %s.", ctx.Err().Error())
			return nil
		case <-time.After(time.Minute):
			continue
		}
	}
}

func SyncLoop(ctx context.Context) {
	glog.Info("SyncLoop start")
	loadCRD(ctx)
	interval := config.GetInt("INTERVAL")
	for {
		select {
		case <-ctx.Done():
			glog.Info("SyncLoop exit")
			return
		case <-time.After(time.Duration(interval) * time.Second):
			// Do nothing
		}
		if config.IsStandalone() {
			interval = 300
			glog.Info("Skip because run in stand alone mode.")
			continue
		}
		interval = config.GetInt("INTERVAL")
		lbsInCloud, err := FetchLBFromMirana2()
		if err != nil {
			glog.Errorf("FetchLoadBalancersInfo() failed:%s", err.Error())
			continue
		}
		lbsInCloud = filterLoadbalancers(lbsInCloud, config.Get("LB_TYPE"), config.Get("NAME"))
		if len(lbsInCloud) == 0 {
			glog.Info("No matched LB found.")
			continue
		}

		albs, err := FetchALBInfo()
		if err != nil {
			glog.Errorf("FetchALBInfo() failed:%s", err.Error())
			continue
		}

		check := map[string]*LBPair{}
		for _, alb := range albs {
			if !lbMatch(alb.Spec, config.Get("LB_TYPE"), config.Get("NAME")) {
				continue
			}
			if _, ok := check[alb.Spec.Name]; !ok {
				check[alb.Spec.Name] = &LBPair{}
			}
			newAlb := alb // MUST copy first
			check[alb.Spec.Name].alb = &newAlb
		}
		for _, lb := range lbsInCloud {
			if _, ok := check[lb.Name]; !ok {
				check[lb.Name] = &LBPair{}
			}
			newLb := *lb
			check[lb.Name].lb = &newLb
		}

		for _, pair := range check {
			if pair.lb != nil {
				if pair.alb == nil {
					AddALB(ctx, pair.lb)
				} else {
					if !reflect.DeepEqual(pair.lb, pair.alb.Spec) {
						UpdateALB(ctx, pair.alb, pair.lb)
					}
				}
			} else {
				if pair.alb != nil {
					DelALB(ctx, pair.alb)
				}
			}
		}
	}
}

func AddALB(ctx context.Context, lb *LoadBalancer) (err error) {
	defer printFuncLog("AddALB", time.Now(), err)
	alb := ALB{
		TypeMeta: metav1.TypeMeta{
			Kind:       "AlaudaLoadBalancer",
			APIVersion: "alauda.io/v3",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: lb.Name,
		},
		Spec: lb,
	}
	data, err := json.Marshal(alb)
	if err != nil {
		glog.Error(err)
		return err
	}
	url := fmt.Sprintf("%s/apis/alauda.io/v3/alaudaloadbalancers", config.Get("KUBERNETES_SERVER"))
	resp, body, errs := GetK8sHTTPClient("POST", url).Send(string(data)).End()
	if len(errs) > 0 {
		glog.Error(errs[0])
		return errs[0]
	}
	// POST will return 201
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		glog.Errorf("Request to %s get %d: %s", url, resp.StatusCode, body)
		return errors.New(body)
	}

	return nil
}

func UpdateALB(ctx context.Context, alb *ALB, lb *LoadBalancer) (err error) {
	defer printFuncLog("UpdateALB", time.Now(), err)
	alb.Spec = lb
	data, err := json.Marshal(alb)
	if err != nil {
		glog.Error(err)
		return
	}

	url := fmt.Sprintf("%s/apis/alauda.io/v3/alaudaloadbalancers/%s",
		config.Get("KUBERNETES_SERVER"),
		lb.Name,
	)
	resp, body, errs := GetK8sHTTPClient("PUT", url).Send(string(data)).End()
	if len(errs) > 0 {
		err = errs[0]
		glog.Error(err)
		return
	}
	// POST will return 201
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		err = errors.New(body)
		glog.Errorf("Request to %s get %d: %s", url, resp.StatusCode, body)
		return
	}
	return nil
}

func DelALB(ctx context.Context, alb *ALB) (err error) {
	defer printFuncLog("DelALB", time.Now(), err)
	url := fmt.Sprintf("%s/apis/alauda.io/v3/alaudaloadbalancers/%s",
		config.Get("KUBERNETES_SERVER"),
		alb.Name,
	)
	resp, body, errs := GetK8sHTTPClient("DELETE", url).End()
	if len(errs) > 0 {
		glog.Error(errs[0])
		return errs[0]
	}
	if resp.StatusCode != 200 {
		glog.Errorf("Request to %s get %d: %s", url, resp.StatusCode, body)
		return errors.New(body)
	}
	return nil
}
