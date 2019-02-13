package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/golang/glog"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"alb2/config"
	"alb2/driver"
	m "alb2/modules"
	alb2v1 "alb2/pkg/apis/alauda/v1"
)

const (
	BindKey    = "loadbalancer.alauda.io/bind"
	ActionBind = "bind"
)

const (
	TypeFrontend = "frontends"
	TypeRule     = "rules"

	StateReady = "ready"
	StateError = "error"
)

// BindInfo [{"container_port": 8080, "protocol": "http", "name": "lb-name", "port": 80}]
type BindInfo struct {
	//Name of alb
	Name          string `json:"name"`
	Port          int    `json:"port"`
	ContainerPort int    `json:"container_port"`
	Protocol      string `json:"protocol"`
	ResourceType  string `json:"resource_type"`
	ResourceName  string `json:"resource_name"`
	State         string `json:"state"`
	ErrorMsg      string `json:"error_message"`
	ServiceName   string `json:"service_name"`
	Namespace     string `json:"namespace"`
}

type Listener struct {
	ServiceID     string   `json:"service_id"`
	ContainerPort int      `json:"container_port"`
	ListenerPort  int      `json:"listener_port"`
	Protocol      string   `json:"protocol"`
	Domains       []string `json:"domains,omitempty"`
}

func (l *Listener) String() string {
	return fmt.Sprintf("%d/%s-%s:%d-%v", l.ListenerPort, l.Protocol, l.ServiceID, l.ContainerPort, l.Domains)
}

func UpdateServiceBind(kd *driver.KubernetesDriver, result *BindInfo) error {
	svc, err := kd.Client.CoreV1().Services(result.Namespace).Get(result.ServiceName, metav1.GetOptions{})
	if err != nil {
		glog.Errorf("Get service %s.%s failed: %s", result.ServiceName, result.Namespace, err)
		return err
	}
	jsonInfo, ok := svc.Annotations[config.Get("labels.bindkey")]
	if !ok {
		glog.Errorf("bind info is not found on service %s.%s", result.ServiceName, result.Namespace)
		return nil //ingore it
	}
	var bindInfos []*BindInfo
	err = json.Unmarshal([]byte(jsonInfo), &bindInfos)
	if err != nil {
		glog.Error(err)
		return err
	}
	found := false
	for idx, b := range bindInfos {
		if b.Name == result.Name && b.Port == result.Port &&
			b.Protocol == result.Protocol && b.ContainerPort == result.ContainerPort {
			bindInfos[idx] = result
			found = true
			break
		}
	}
	if found {
		js, err := json.Marshal(bindInfos)
		if err != nil {
			glog.Error(err)
		}
		svc.Annotations[config.Get("labels.bindkey")] = string(js)
		svc, err = kd.Client.CoreV1().Services(result.Namespace).Update(svc)
		if err != nil {
			glog.Errorf("Update service %s.%s failed: %s", result.ServiceName, result.Namespace, err)
			return err
		}
	} else {
		glog.Infof("No matched bind info found for %+v", *result)
	}
	return nil
}

func ListBindRequest(kd *driver.KubernetesDriver) ([]*BindInfo, error) {
	serviceList, err := kd.Client.CoreV1().Services("").List(metav1.ListOptions{})
	if err != nil {
		glog.Error(err)
		return nil, err
	}
	result := []*BindInfo{}

	for _, svc := range serviceList.Items {
		jsonInfo, ok := svc.Annotations[config.Get("labels.bindkey")]
		if !ok {
			continue
		}
		var bindInfos []*BindInfo
		err = json.Unmarshal([]byte(jsonInfo), &bindInfos)
		if err != nil {
			glog.Error(err)
			continue
		}
		lbname := config.Get("NAME")
		for _, b := range bindInfos {
			if b.Port <= 0 || b.Port > 65535 {
				continue
			}
			if b.Name == lbname &&
				b.State != StateReady {
				b.ServiceName = svc.Name
				b.Namespace = svc.Namespace
				result = append(result, b)
			}
		}
	}
	return result, nil
}

func bindTcp(alb *m.AlaudaLoadBalancer, req *BindInfo) (*BindInfo, error) {
	d, err := driver.GetDriver()
	if err != nil {
		return nil, err
	}
	result := *req //copy for modify
	var ft *m.Frontend
	for _, ft = range alb.Frontends {
		if ft.Port == result.Port {
			break
		}
	}
	if ft == nil || ft.Port != result.Port {
		// no frontend found
		ft = nil
	}
	if ft == nil {
		ft, err = alb.NewFrontend(result.Port, result.Protocol)
		if err != nil {
			glog.Error(err)
			return nil, err
		}
	}

	if ft.ServiceGroup == nil {
		ft.ServiceGroup = &alb2v1.ServiceGroup{
			Services: []alb2v1.Service{},
		}
	}
	glog.Infof("ft %+v has service: %+v", *ft, ft.ServiceGroup.Services)
	if len(ft.ServiceGroup.Services) > 0 {
		if ft.ServiceGroup.Services[0].Is(result.Namespace, result.ServiceName, result.ContainerPort) {
			result.State = StateReady
			result.ErrorMsg = ""
		} else {
			glog.Infof("frontend is used by another service")
			result.State = StateError
			result.ErrorMsg = "frontend is used by another service"
		}
		return &result, nil
	}

	ft.Source = &alb2v1.Source{
		Type:      m.TypeBind,
		Name:      result.ServiceName,
		Namespace: result.Namespace,
	}

	ft.ServiceGroup.Services = append(
		ft.ServiceGroup.Services,
		alb2v1.Service{
			Namespace: result.Namespace,
			Name:      result.ServiceName,
			Port:      result.ContainerPort,
			Weight:    100,
		},
	)
	err = d.UpsertFrontends(alb, ft)
	if err != nil {
		glog.Errorf("upsert ft failed: %s", err)
		return nil, err
	}
	result.ResourceType = "frontends"
	result.ResourceName = ft.Name
	result.State = StateReady
	glog.Infof("bind tcp service %s.%s:%d to %s:%d success",
		result.Name, result.Namespace, result.ContainerPort,
		result.Name, result.Port,
	)
	return &result, nil
}

func bindHTTP(alb *m.AlaudaLoadBalancer, req *BindInfo) (*BindInfo, error) {
	d, err := driver.GetDriver()
	if err != nil {
		return nil, err
	}
	result := *req //copy for modify
	var ft *m.Frontend
	for _, ft = range alb.Frontends {
		if ft.Port == result.Port {
			break
		}
	}
	if ft == nil || ft.Port != result.Port {
		// no frontend found
		ft = nil
	}
	if ft == nil {
		ft, err = alb.NewFrontend(result.Port, result.Protocol)
		if err != nil {
			glog.Error(err)
			return nil, err
		}
		err := d.UpsertFrontends(alb, ft)
		if err != nil {
			glog.Error(err)
			return nil, err
		}
	}
	if ft.Protocol != req.Protocol {
		result.State = StateError
		result.ErrorMsg = fmt.Sprintf("Frontend has differnet protocol")
		return &result, nil
	}

	domains := alb.ListDomains()
	if len(domains) == 0 {
		glog.Infof(
			"Can't handle bind request on %s.%s: no domain suffix on alb %s",
			result.ServiceName, result.Namespace, alb.Name,
		)
		result.State = StateError
		result.ErrorMsg = "no domain suffix on alb"
	}

domainLoop:
	for _, ds := range alb.ListDomains() {
		domain := fmt.Sprintf("%s.%s.%s", result.ServiceName, result.Namespace, ds)

		for _, rule := range ft.Rules {
			if rule.Domain == domain && rule.URL == "" {
				result.ResourceType = TypeRule
				result.ResourceName = rule.Name
				result.State = StateReady
				continue domainLoop
			}
		}

		r, _ := ft.NewRule(domain, "", "", "", "")
		r.Source = &alb2v1.Source{
			Type:      m.TypeBind,
			Name:      result.ServiceName,
			Namespace: result.Namespace,
		}
		r.ServiceGroup = &alb2v1.ServiceGroup{
			Services: []alb2v1.Service{
				alb2v1.Service{
					Name:      result.ServiceName,
					Namespace: result.Namespace,
					Port:      result.ContainerPort,
					Weight:    100,
				},
			},
		}
		err := d.CreateRule(r)
		if err != nil {
			glog.Error(err)
			continue
		}
	}
	return &result, nil
}

func Bind(alb *m.AlaudaLoadBalancer, req *BindInfo) (*BindInfo, error) {
	switch req.Protocol {
	case ProtocolTCP:
		return bindTcp(alb, req)
	case ProtocolHTTP:
		return bindHTTP(alb, req)
	case ProtocolHTTPS:
		// TOOD: support https
		return nil, fmt.Errorf("unknown protocol %s", req.Protocol)
	default:
		glog.Errorf("Find unknown protocol %s from bind request %s.%s",
			req.Protocol, req.ServiceName, req.Namespace)
		return nil, fmt.Errorf("unknown protocol %s", req.Protocol)
	}
}

func RegisterLoop(ctx context.Context) {
	glog.Info("RegisterLoop start")
	kd, err := driver.GetDriver()
	if err != nil {
		glog.Fatal(err)
	}
	interval := config.GetInt("INTERVAL")*2 + 1
	for {
		select {
		case <-ctx.Done():
			glog.Infof("RegisterLoop exit because %s.", ctx.Err().Error())
			return
		case <-time.After(time.Duration(interval) * time.Second): //sleep
		}

		interval = config.GetInt("INTERVAL")*2 + 1

		err := TryLockAlb()
		if err != nil {
			continue
		}

		alb, err := kd.LoadALBbyName(
			config.Get("NAMESPACE"),
			config.Get("NAME"),
		)
		if err != nil {
			glog.Error(err)
			continue
		}

		bindRequest, err := ListBindRequest(kd)
		if err != nil {
			glog.Error(err)
			continue
		}
		if len(bindRequest) == 0 {
			glog.Info("No bind request")
			continue
		}
		glog.Infof("There are %d bind requests need to process.", len(bindRequest))

		for _, req := range bindRequest {
			// make sure do not call api server too frequently
			time.Sleep(200 * time.Millisecond)
			glog.Infof("Try to bind %+v", *req)
			result, err := Bind(alb, req)
			if err != nil {
				glog.Errorf(
					"bind %s.%s:%d to %s:%d failed: %s",
					req.ServiceName, req.Namespace, req.ContainerPort,
					req.Name, req.Port, err.Error(),
				)
			} else {
				glog.Infof("get bind result %+v", *result)
			}
			if result.State != req.State || result.ErrorMsg != req.ErrorMsg {
				// Update service
				glog.Infof("Update bind info of %s.%s", result.ServiceName, result.Namespace)
				UpdateServiceBind(kd, result)
			}
		}
	}
}
