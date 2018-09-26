package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/golang/glog"

	typev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"alb2/config"
	"alb2/driver"
)

const (
	BindKey    = "loadbalancer.alauda.io/bind"
	ActionBind = "bind"
)

// BindInfo [{"container_port": 8080, "protocol": "http", "name": "lb-name", "port": 80}]
type BindInfo struct {
	Name          string `json:"name"`
	Port          int    `json:"port"`
	ContainerPort int    `json:"container_port"`
	Protocol      string `json:"protocol"`
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

type BindRequest struct {
	Action    string      `json:"action"`
	Listeners []*Listener `json:"listeners"`

	loadbalancerID string
}

func GetBindingService(kd *driver.KubernetesDriver) (bindMap map[string]map[string][]*Listener, err error) {
	pods, err := kd.Client.CoreV1().Pods("").List(metav1.ListOptions{
		LabelSelector: config.Get("LABEL_SERVICE_ID"),
	})
	if err != nil {
		glog.Error(err)
		return nil, err
	}
	podsMap := make(map[string]bool)
	for _, pod := range pods.Items {
		if pod.Status.Phase != typev1.PodRunning {
			continue
		}
		sid := pod.Labels[config.Get("LABEL_SERVICE_ID")]
		if sid != "" {
			podsMap[sid] = true
		}
	}
	serviceList, err := kd.Client.CoreV1().Services("").List(metav1.ListOptions{
		// service should have LABEL_SERVICE_ID and not have LABEL_CREATOR
		LabelSelector: fmt.Sprintf("%s,!%s", config.Get("LABEL_SERVICE_ID"), config.Get("LABEL_CREATOR")),
	})
	if err != nil {
		glog.Error(err)
		return nil, err
	}
	// { lb-name: {service_id: [listeners]}}
	bindMap = make(map[string]map[string][]*Listener)
	for _, svc := range serviceList.Items {
		jsonInfo, ok := svc.Annotations[BindKey]
		if !ok {
			continue
		}
		serviceID := svc.Labels[config.Get("LABEL_SERVICE_ID")]
		if serviceID == "" {
			glog.Errorf("svc %s/%s had empty service id.", svc.Namespace, svc.Name)
			continue
		}
		if _, ok := podsMap[serviceID]; !ok {
			glog.Infof("service %s has no backend, skip it", serviceID)
			continue
		}
		var bindInfos []BindInfo
		err = json.Unmarshal([]byte(jsonInfo), &bindInfos)
		if err != nil {
			glog.Error(err)
			continue
		}

	loop:
		for _, bindInfo := range bindInfos {
			if bindInfo.Port <= 0 || bindInfo.Port > 65535 {
				continue
			}
			svcMap, ok := bindMap[bindInfo.Name]
			if !ok {
				svcMap = make(map[string][]*Listener)
				bindMap[bindInfo.Name] = svcMap
			}

			domain := fmt.Sprintf("%s.%s.{LB_DOMAINS}", svc.Name, svc.Namespace)

			for _, l := range svcMap[serviceID] {
				if l.ContainerPort == bindInfo.ContainerPort &&
					l.ListenerPort == bindInfo.Port &&
					l.Protocol == bindInfo.Protocol {
					l.Domains = append(l.Domains, domain)
					continue loop
				}
			}
			listener := &Listener{
				ServiceID:     serviceID,
				ContainerPort: bindInfo.ContainerPort,
				ListenerPort:  bindInfo.Port,
				Protocol:      bindInfo.Protocol,
				Domains: []string{
					fmt.Sprintf("%s.%s.{LB_DOMAINS}", svc.Name, svc.Namespace),
				},
			}
			svcMap[serviceID] = append(svcMap[serviceID], listener)
		}

	}
	return bindMap, nil
}

func NeedUpdate(lb *LoadBalancer, listeners []*Listener) bool {
	for _, l := range listeners {
		var frontend *Frontend
		for _, ft := range lb.Frontends {
			if ft.Port == l.ListenerPort {
				frontend = ft
				break
			}
		}
		if frontend == nil {
			glog.Infof("frontend %d doesn't exist yet", l.ListenerPort)
			return true
		}
		switch l.Protocol {
		case ProtocolHTTP:
		domainLoop:
			for _, domain := range l.Domains {
				prefix := strings.Replace(domain, "{LB_DOMAINS}", "", 1)
				for _, rule := range frontend.Rules {
					if rule.Type == "system" &&
						strings.HasPrefix(rule.Domain, prefix) {
						continue domainLoop
					}
				}
				glog.Infof("Doamin %s need to bind", domain)
				return true
			}
		case ProtocolTCP:
			if frontend.ServiceID == "" {
				return true
			}
			if frontend.ServiceID == l.ServiceID &&
				frontend.ContainerPort != l.ContainerPort {
				return true
			}
			if frontend.ServiceID != l.ServiceID {
				glog.Infof("Port %d on LB %s is already used by %s.",
					frontend.Port,
					lb.Name,
					frontend.ServiceID,
				)
			}
		}
	}
	return false
}

func BindService(req *BindRequest) {
	url := fmt.Sprintf("%s/v1/load_balancers/%s/%s",
		config.Get("JAKIRO_ENDPOINT"),
		config.Get("NAMESPACE"),
		req.loadbalancerID,
	)
	data, err := json.Marshal(req)
	if err != nil {
		glog.Error(err)
		return
	}
	glog.Infof("try to send bind request: url = %s, body= %s", url, string(data))
	resp, body, errs := JakiroRequest.Put(url).Send(string(data)).
		Set("Authorization", fmt.Sprintf("Token %s", config.Get("TOKEN"))).
		Timeout(time.Second * 15).
		End()
	if len(errs) > 0 {
		glog.Error(errs[0].Error())
		return
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		glog.Errorf("Request to %s get %d: %s", url, resp.StatusCode, body)
		return
	}
	glog.Info("bind success.")
}

func hasDomainSuffix(lb *LoadBalancer) bool {
	if lb == nil {
		return false
	}
	for _, domain := range lb.DomainInfo {
		if !domain.Disabled {
			return true
		}
	}
	return false
}

func RegisterLoop(ctx context.Context) {
	glog.Info("RegisterLoop start")
	timeout, err := strconv.Atoi(config.Get("KUBERNETES_TIMEOUT"))
	if err != nil {
		timeout = 30
	}
	kd, err := driver.GetKubernetesDriver(
		config.Get("KUBERNETES_SERVER"),
		config.Get("KUBERNETES_BEARERTOKEN"), timeout)
	if err != nil {
		glog.Error(err)
		glog.Flush()
		panic(err)
	}
	interval := config.GetInt("INTERVAL") * 3
	for {
		select {
		case <-ctx.Done():
			glog.Infof("RegisterLoop exit because %s.", ctx.Err().Error())
			return
		case <-time.After(time.Duration(interval) * time.Second): //sleep
		}
		if config.IsStandalone() {
			interval = 300
			glog.Info("Skip because run in stand alone mode.")
			continue
		}
		interval = config.GetInt("INTERVAL") * 3
		bindMap, err := GetBindingService(kd)
		if err != nil {
			glog.Error(err)
			continue
		}
		if len(bindMap) == 0 {
			continue
		}

		lbs, err := FetchLoadBalancersInfo()
		if err != nil {
			glog.Error(err)
			continue
		}

		lbs = filterLoadbalancers(lbs, config.Get("LB_TYPE"), config.Get("NAME"))
		if len(lbs) == 0 {
			glog.Info("No matched LB found.")
			continue
		}

		requests := []*BindRequest{}
		for name, svcMap := range bindMap {
			var loadbalancer *LoadBalancer
			for _, lb := range lbs {
				if lb.Name == name {
					loadbalancer = lb
					break
				}
			}
			if loadbalancer == nil {
				continue
			}
			if !hasDomainSuffix(loadbalancer) {
				glog.Infof("LB %s doesn't have any domain suffix, skip it.", loadbalancer.Name)
				continue
			}

			for _, listeners := range svcMap {
				if NeedUpdate(loadbalancer, listeners) {
					req := &BindRequest{
						Action:         "bind",
						Listeners:      listeners,
						loadbalancerID: loadbalancer.LoadBalancerID,
					}
					requests = append(requests, req)
				}
			}
		} //for name, svcMap := range bindMap

		for _, req := range requests {
			BindService(req)
		}
	}
}
