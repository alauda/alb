package controller

import (
	m "alb2/modules"

	"github.com/golang/glog"
)

func MergeNew(alb *m.AlaudaLoadBalancer) (*LoadBalancer, error) {
	lb := &LoadBalancer{
		Name:           alb.Name,
		Address:        alb.Address,
		BindAddress:    alb.BindAddress,
		LoadBalancerID: alb.LoadBalancerID,
		Frontends:      []*Frontend{},
	}
	if lb.BindAddress == "" {
		lb.BindAddress = "*"
	}
	for _, aft := range alb.Frontends {
		ft := &Frontend{
			LoadBalancerID:  alb.Name,
			Port:            aft.Port,
			Protocol:        aft.Protocol,
			CertificateID:   aft.CertificateID,
			CertificateName: aft.CertificateName,
			Rules:           RuleList{},
		}
		if ft.Protocol == "" {
			ft.Protocol = ProtocolTCP
		}
		if ft.Port <= 0 {
			glog.Errorf("frontend %s has an invalid port %d", aft.Name, aft.Port)
		}
		for idx, arl := range aft.Rules {
			rule := &Rule{
				RuleID:      arl.Name,
				Priority:    arl.Priority * int64(idx+1),
				Type:        arl.Type,
				Domain:      arl.Domain,
				URL:         arl.URL,
				DSL:         arl.DSL,
				Description: arl.Description,
			}
			if arl.ServiceGroup != nil {
				rule.SessionAffinityPolicy = arl.ServiceGroup.SessionAffinityPolicy
				rule.SessionAffinityAttr = arl.ServiceGroup.SessionAffinityAttribute
				for _, svc := range arl.ServiceGroup.Services {
					if rule.Services == nil {
						rule.Services = []*BackendService{}
					}
					rule.Services = append(rule.Services, &BackendService{
						ServiceID:     svc.ServiceID(),
						ContainerPort: svc.Port,
						Weight:        svc.Weight,
					})
				}
			}
			ft.Rules = append(ft.Rules, rule)
		}
		if aft.ServiceGroup != nil {
			for _, svc := range aft.ServiceGroup.Services {
				ft.ServiceID = svc.ServiceID()
				ft.ContainerPort = svc.Port
			}
		}

		lb.Frontends = append(lb.Frontends, ft)
	}
	return lb, nil
}
