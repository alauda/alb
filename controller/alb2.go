package controller

import (
	"fmt"

	"github.com/golang/glog"

	d "alb2/driver"
	m "alb2/modules"
)

func getServiceKey(ns, name string, port int) string {
	return fmt.Sprintf("%s-%s-%d", ns, name, port)
}

func parseServiceGroup(data map[string]*d.Service, sg *m.ServicceGroup) (map[string]*d.Service, error) {
	if sg == nil {
		return data, nil
	}

	kd, err := d.GetDriver()
	if err != nil {
		glog.Error(err)
		return data, err
	}

	for _, svc := range sg.Services {
		key := svc.String()
		if _, ok := data[key]; !ok {
			backend, err := kd.GetServiceByName(svc.Namespace, svc.Name, svc.Port)
			if err != nil {
				glog.Error(err)
				continue
			}
			data[key] = backend
		}
	}
	return data, nil
}

func LoadServices(alb *m.AlaudaLoadBalancer) (map[string]*d.Service, error) {
	var err error
	data := make(map[string]*d.Service)

	for _, ft := range alb.Frontends {
		data, err = parseServiceGroup(data, ft.ServiceGroup)
		if err != nil {
			glog.Error(err)
			return nil, err
		}

		for _, rule := range ft.Rules {
			data, err = parseServiceGroup(data, ft.ServiceGroup)
			if err != nil {
				glog.Error(err)
				return nil, err
			}
		}
	}
	return data, nil
}

func MergeNew(alb *m.AlaudaLoadBalancer) (*LoadBalancer, error) {
	lb := &LoadBalancer{
		Name:           alb.Name,
		Address:        alb.Address,
		BindAddress:    alb.BindAddress,
		LoadBalancerID: alb.LoadBalancerID,
		Frontends:      []*Frontend{},
	}
	for _, aft := range alb.Frontends {
		ft := &Frontend{
			LoadBalancerID:  alb.LoadBalancerID,
			Port:            aft.Port,
			Protocol:        aft.Protocol,
			CertificateID:   aft.CertificateID,
			CertificateName: aft.CertificateName,
			Rules:           RuleList{},
		}
		for _, arl := range aft.Rules {
			rule := &Rule{
				RuleID:      arl.Name,
				Priority:    arl.Priority,
				Type:        arl.Type,
				Domain:      arl.Domain,
				URL:         arl.URL,
				DSL:         arl.DSL,
				Description: arl.Description,
			}
			if arl.ServiceGroup != nil {
				rule.SessionAffinityPolicy = arl.ServiceGroup.SessionAffinityPolicy
				rule.SessionAffinityAttr = arl.ServiceGroup.SessionAffinityAttribute
			}
			ft.Rules = append(ft.Rules, rule)
		}
		lb.Frontends = append(lb.Frontends, ft)
	}
	return lb, nil
}
