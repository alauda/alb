package cli

import (
	"strings"

	. "alauda.io/alb2/controller/custom_config"
	. "alauda.io/alb2/controller/types"
	"alauda.io/alb2/driver"
	albv1 "alauda.io/alb2/pkg/apis/alauda/v1"
	"github.com/go-logr/logr"
	"k8s.io/klog/v2"
)

// cli to fetch loadbalancer from alb/ft/rule
type AlbCli struct {
	drv *driver.KubernetesDriver
	log logr.Logger
}

func NewAlbCli(drv *driver.KubernetesDriver, log logr.Logger) AlbCli {
	return AlbCli{
		drv: drv,
		log: log,
	}
}

func (c *AlbCli) GetLBConfig(ns string, name string) (*LoadBalancer, error) {
	// TODO we should merge mAlb and cAlb to one struct.
	// mAlb LoadBalancer struct from modules package used in driver.
	// cAlb LoadBalancer struct from controller package.
	kd := *c.drv
	mAlb, err := kd.LoadALBbyName(ns, name)
	if err != nil {
		klog.Error("load mAlb fail", err)
		return nil, err
	}

	cAlb := &LoadBalancer{
		Name:      mAlb.Name,
		Address:   mAlb.Spec.Address,
		Frontends: []*Frontend{},
		Labels:    mAlb.Labels,
	}

	// mft frontend struct from modules package.
	for _, mft := range mAlb.Frontends {
		ft := &Frontend{
			FtName:          mft.Name,
			AlbName:         mAlb.Name,
			Port:            mft.Spec.Port,
			Protocol:        mft.Spec.Protocol,
			Rules:           RuleList{},
			CertificateName: mft.Spec.CertificateName,
			BackendProtocol: mft.Spec.BackendProtocol,
			Labels:          mft.Labels,
		}
		if !ft.IsValidProtocol() {
			klog.Errorf("frontend %s %s has no valid protocol", ft.FtName, ft.Protocol)
			ft.Protocol = albv1.FtProtocolTCP
		}

		if ft.Port <= 0 {
			klog.Errorf("frontend %s has an invalid port %d", ft.FtName, ft.Port)
			continue
		}

		// translate rule cr to our rule struct
		for _, marl := range mft.Rules {
			arl := marl.Spec
			ruleConfig := RuleConfigFromRuleAnnotation(marl.Name, marl.Annotations, c.drv.Opt.Domain)
			rule := &Rule{
				Config:           ruleConfig,
				RuleID:           marl.Name,
				Priority:         arl.Priority,
				Type:             arl.Type,
				Domain:           strings.ToLower(arl.Domain),
				URL:              arl.URL,
				DSLX:             arl.DSLX,
				Description:      arl.Description,
				CertificateName:  arl.CertificateName,
				RewriteBase:      arl.RewriteBase,
				RewriteTarget:    arl.RewriteTarget,
				EnableCORS:       arl.EnableCORS,
				CORSAllowHeaders: arl.CORSAllowHeaders,
				CORSAllowOrigin:  arl.CORSAllowOrigin,
				BackendProtocol:  arl.BackendProtocol,
				RedirectURL:      arl.RedirectURL,
				RedirectCode:     arl.RedirectCode,
				VHost:            arl.VHost,
				Source:           arl.Source,
			}
			if arl.ServiceGroup != nil {
				rule.SessionAffinityPolicy = arl.ServiceGroup.SessionAffinityPolicy
				rule.SessionAffinityAttr = arl.ServiceGroup.SessionAffinityAttribute
				if rule.Services == nil {
					rule.Services = []*BackendService{}
				}
				for _, svc := range arl.ServiceGroup.Services {
					rule.Services = append(rule.Services, &BackendService{
						ServiceNs:   svc.Namespace,
						ServiceName: svc.Name,
						ServicePort: svc.Port,
						Weight:      svc.Weight,
					})
				}
			}
			ft.Rules = append(ft.Rules, rule)
		}

		if mft.Spec.ServiceGroup != nil {
			ft.Services = []*BackendService{}
			ft.BackendGroup = &BackendGroup{
				Name:                     ft.String(),
				SessionAffinityAttribute: mft.Spec.ServiceGroup.SessionAffinityAttribute,
				SessionAffinityPolicy:    mft.Spec.ServiceGroup.SessionAffinityPolicy,
			}

			for _, svc := range mft.Spec.ServiceGroup.Services {
				ft.Services = append(ft.Services, &BackendService{
					ServiceNs:   svc.Namespace,
					ServiceName: svc.Name,
					ServicePort: svc.Port,
					Weight:      svc.Weight,
				})
			}
		}

		cAlb.Frontends = append(cAlb.Frontends, ft)
	}
	return cAlb, nil
}
