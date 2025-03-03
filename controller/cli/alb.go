package cli

import (
	"strings"
	"time"

	"alauda.io/alb2/controller/modules"
	. "alauda.io/alb2/controller/types"
	"alauda.io/alb2/driver"
	albv1 "alauda.io/alb2/pkg/apis/alauda/v1"
	ext "alauda.io/alb2/pkg/controller/extctl"
	pm "alauda.io/alb2/pkg/utils/metrics"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// cli to fetch loadbalancer from alb/ft/rule
type AlbCli struct {
	drv *driver.KubernetesDriver
	log logr.Logger
	cus ext.ExtCtl
}

func NewAlbCli(drv *driver.KubernetesDriver, log logr.Logger) AlbCli {
	return AlbCli{
		drv: drv,
		log: log,
		cus: ext.NewExtensionCtl(ext.ExtCtlCfgOpt{
			Log:    log,
			Domain: drv.Opt.Domain,
		}),
	}
}

func (c *AlbCli) RuleToInternalRule(mr *modules.Rule, ir *InternalRule) {
	mrs := mr.Spec

	// rule-meta
	meta := RuleMeta{}
	meta.RuleID = mr.Name
	meta.Type = mrs.Type
	meta.Source = mrs.Source
	meta.Priority = mrs.Priority
	ir.RuleMeta = meta

	// rule-match
	match := RuleMatch{}
	match.DSLX = mrs.DSLX
	ir.RuleMatch = match

	// rule-cert
	cert := RuleCert{}
	cert.CertificateName = mrs.CertificateName
	cert.Domain = mrs.Domain
	ir.RuleCert = cert

	// rule-upstream
	// 在redirect的情况下，service group为null 是正常的
	if mrs.ServiceGroup != nil {
		up := RuleUpstream{}
		up.BackendProtocol = strings.ToLower(mrs.BackendProtocol)
		up.SessionAffinityPolicy = mrs.ServiceGroup.SessionAffinityPolicy
		up.SessionAffinityAttr = mrs.ServiceGroup.SessionAffinityAttribute
		if up.Services == nil {
			up.Services = []*BackendService{}
		}
		for _, svc := range mrs.ServiceGroup.Services {
			up.Services = append(up.Services, &BackendService{
				ServiceNs:   svc.Namespace,
				ServiceName: svc.Name,
				ServicePort: svc.Port,
				Weight:      svc.Weight,
			})
		}
		// will be init in fillup backend phase
		up.BackendGroup = &BackendGroup{}
		ir.RuleUpstream = up
	}
	ext := RuleExt{
		Source: make(ConfigSource),
	}
	ir.Config = ext
	// 为了方便，我们给rule-config一个默认值
	if mr.Spec.Config == nil {
		mr.Spec.Config = &albv1.RuleConfigInCr{}
	}
	// rule-ext
	c.cus.ToInternalRule(mr, ir)
}

func (c *AlbCli) GetLBConfig(ns string, name string) (*LoadBalancer, error) {
	kd := *c.drv
	s := time.Now()
	mAlb, err := kd.LoadALBbyName(ns, name)
	if err != nil {
		c.log.Error(err, "load m-alb fail")
		return nil, err
	}
	pm.Write("load-m-alb", float64(time.Since(s).Milliseconds()))

	cAlb := &LoadBalancer{
		Name:        mAlb.Alb.Name,
		Annotations: mAlb.Alb.Annotations,
		Address:     mAlb.Alb.Spec.Address,
		Frontends:   []*Frontend{},
		Labels:      mAlb.Alb.Labels,
	}

	c.log.Info("ft len", "alb", name, "ft", len(mAlb.Frontends))
	// mft frontend struct from modules package.
	for _, mft := range mAlb.Frontends {
		if mft.Spec.Config == nil {
			mft.Spec.Config = &albv1.FTConfig{}
		}
		ft := &Frontend{
			FtName:          mft.Name,
			AlbName:         mAlb.Alb.Name,
			Port:            mft.Spec.Port,
			Protocol:        mft.Spec.Protocol,
			Rules:           RuleList{},
			CertificateName: mft.Spec.CertificateName,
			BackendProtocol: strings.ToLower(mft.Spec.BackendProtocol),
			Labels:          mft.Labels,
		}
		if !ft.IsValidProtocol() {
			c.log.Info("frontend has no valid protocol", "ft", ft.FtName, "protocol", ft.Protocol)
			ft.Protocol = albv1.FtProtocolTCP
		}

		if ft.Port <= 0 {
			c.log.Info("frontend has an invalid port ", "ft", ft.FtName, "port", ft.Port)
			continue
		}

		c.log.Info("rule", "ft", ft.FtName, "rule", len(mft.Rules))
		// translate rule cr to our rule struct
		for _, marl := range mft.Rules {
			rule := &InternalRule{}
			c.RuleToInternalRule(marl, rule)
			ft.Rules = append(ft.Rules, rule)
		}

		if mft.Spec.ServiceGroup != nil {
			ft.Services = []*BackendService{}
			// @ft_default_policy
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
		// l4
		if ft.IsStreamMode() {
			c.cus.InitL4Ft(mft, ft)
		} else {
			c.cus.InitL7Ft(mft, ft)
		}
		cAlb.Frontends = append(cAlb.Frontends, ft)
	}
	cAlb.Refs = RefMap{
		ConfigMap: map[client.ObjectKey]*corev1.ConfigMap{},
		Secret:    map[client.ObjectKey]*corev1.Secret{},
	}
	return cAlb, nil
}

func (c *AlbCli) CollectAndFetchRefs(lb *LoadBalancer) {
	s := time.Now()
	defer func() {
		e := time.Now()
		pm.Write("collect-refs", float64(e.UnixMilli())-float64(s.UnixMilli()))
	}()
	s_pick_refs := time.Now()
	for _, ft := range lb.Frontends {
		for _, rule := range ft.Rules {
			c.cus.CollectRefs(rule, lb.Refs)
		}
	}
	pm.Write("collect-refs/pick-refs", float64(time.Since(s_pick_refs).Milliseconds()))
	for k := range lb.Refs.ConfigMap {
		cm := &corev1.ConfigMap{}
		err := c.drv.Cli.Get(c.drv.Ctx, k, cm)
		if err != nil {
			c.log.Error(err, "get cm fail", "cm", k)
			delete(lb.Refs.ConfigMap, k)
			continue
		}
		lb.Refs.ConfigMap[k] = cm
	}
	for k := range lb.Refs.Secret {
		secret := &corev1.Secret{}
		err := c.drv.Cli.Get(c.drv.Ctx, k, secret)
		if err != nil {
			c.log.Error(err, "get secret fail", "secret", k)
			delete(lb.Refs.Secret, k)
			continue
		}
		lb.Refs.Secret[k] = secret
	}
}
