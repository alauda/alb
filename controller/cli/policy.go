package cli

import (
	"fmt"
	"math"
	"net"
	"sort"
	"strings"
	"sync"

	m "alauda.io/alb2/controller/modules"
	"alauda.io/alb2/controller/types"
	. "alauda.io/alb2/controller/types"
	"alauda.io/alb2/driver"
	albv1 "alauda.io/alb2/pkg/apis/alauda/v1"
	"alauda.io/alb2/utils"
	"github.com/go-logr/logr"
	"github.com/thoas/go-funk"
	certutil "k8s.io/client-go/util/cert"
	"k8s.io/klog/v2"

	corev1 "k8s.io/api/core/v1"
)

type PolicyCli struct {
	drv *driver.KubernetesDriver
	log logr.Logger
	opt PolicyCliOpt
}
type PolicyCliOpt struct {
	MetricsPort int
}

func NewPolicyCli(drv *driver.KubernetesDriver, log logr.Logger, opt PolicyCliOpt) PolicyCli {
	return PolicyCli{
		drv: drv,
		log: log,
		opt: opt,
	}
}

// fetch cert and backend info that lb config need, constructs a "dynamic config" used by openresty.
func (p *PolicyCli) GenerateAlbPolicy(alb *LoadBalancer) NgxPolicy {
	certificateMap := getCertMap(alb, p.drv)
	p.setMetricsPortCert(certificateMap)
	backendGroup := pickAllBackendGroup(alb)

	ngxPolicy := NgxPolicy{
		CertificateMap: certificateMap,
		Http:           HttpPolicy{Tcp: make(map[albv1.PortNumber]Policies)},
		Stream:         StreamPolicy{Tcp: make(map[albv1.PortNumber]Policies), Udp: make(map[albv1.PortNumber]Policies)},
		BackendGroup:   backendGroup,
	}

	for _, ft := range alb.Frontends {
		if ft.Conflict {
			continue
		}
		if ft.IsStreamMode() {
			p.initStreamModeFt(ft, &ngxPolicy)
		}

		if ft.IsHttpMode() {
			p.initHttpModeFt(ft, &ngxPolicy)
		}

		if ft.IsGRPCMode() {
			p.initGRPCModeFt(ft, &ngxPolicy)
		}
	}

	return ngxPolicy
}

func pickAllBackendGroup(alb *LoadBalancer) BackendGroups {
	backendGroup := BackendGroups{}
	for _, ft := range alb.Frontends {
		if ft.Conflict {
			continue
		}
		for _, rule := range ft.Rules {
			backendGroup = append(backendGroup, rule.BackendGroup)
		}

		if ft.BackendGroup != nil && len(ft.BackendGroup.Backends) > 0 {
			// FIX: http://jira.alaudatech.com/browse/DEV-16954
			// remove duplicate upstream
			if !funk.Contains(backendGroup, ft.BackendGroup) {
				backendGroup = append(backendGroup, ft.BackendGroup)
			}
		}
	}
	sort.Sort(backendGroup)
	return backendGroup
}

func (p *PolicyCli) initStreamModeFt(ft *Frontend, ngxPolicy *NgxPolicy) {
	// create a default rule for stream mode ft.
	if len(ft.Rules) == 0 {
		if ft.BackendGroup == nil || ft.BackendGroup.Backends == nil {
			klog.Warningf("ft %s,stream mode ft must have backend group", ft.FtName)
		}
		if ft.Protocol == albv1.FtProtocolTCP {
			policy := Policy{}
			policy.Subsystem = SubsystemStream
			policy.Upstream = ft.BackendGroup.Name
			policy.Rule = ft.BackendGroup.Name
			ngxPolicy.Stream.Tcp[ft.Port] = append(ngxPolicy.Stream.Tcp[ft.Port], &policy)
		}
		if ft.Protocol == albv1.FtProtocolUDP {
			policy := Policy{}
			policy.Subsystem = SubsystemStream
			policy.Upstream = ft.BackendGroup.Name
			policy.Rule = ft.BackendGroup.Name
			ngxPolicy.Stream.Udp[ft.Port] = append(ngxPolicy.Stream.Udp[ft.Port], &policy)
		}
		return
	}

	if len(ft.Rules) != 1 {
		klog.Warningf("stream mode ft could only have one rule", ft.FtName)
	}
	rule := ft.Rules[0]
	policy := Policy{}
	policy.Subsystem = SubsystemStream
	policy.Upstream = rule.BackendGroup.Name
	policy.Rule = rule.RuleID
	policy.Config = rule.Config
	if ft.Protocol == albv1.FtProtocolTCP {
		ngxPolicy.Stream.Tcp[ft.Port] = append(ngxPolicy.Stream.Tcp[ft.Port], &policy)
	}
	if ft.Protocol == albv1.FtProtocolUDP {
		ngxPolicy.Stream.Udp[ft.Port] = append(ngxPolicy.Stream.Udp[ft.Port], &policy)
	}
}

func (p *PolicyCli) initHttpModeFt(ft *Frontend, ngxPolicy *NgxPolicy) {
	if _, ok := ngxPolicy.Http.Tcp[ft.Port]; !ok {
		ngxPolicy.Http.Tcp[ft.Port] = Policies{}
	}

	for _, rule := range ft.Rules {
		if rule.DSLX == nil {
			klog.Warningf("rule %s has no matcher, skip", rule.RuleID)
			continue
		}

		klog.V(3).Infof("Rule is %v", rule)
		// translate our rule struct to policy (the json file)
		policy := Policy{}
		policy.Subsystem = SubsystemHTTP
		policy.Rule = rule.RuleID
		policy.DSL = rule.DSLX
		internalDSL, err := utils.DSLX2Internal(rule.DSLX)
		if err != nil {
			klog.Error("convert dslx to internal failed", err)
			continue
		}
		policy.InternalDSL = internalDSL

		policy.Priority = rule.GetPriority()
		policy.RawPriority = rule.GetRawPriority()
		policy.InternalDSLLen = utils.InternalDSLLen(internalDSL)
		// policy-gen 设置 rule 的 upstream
		policy.Upstream = rule.BackendGroup.Name // IMPORTANT
		// for rewrite
		policy.URL = rule.URL
		policy.RewriteBase = rule.RewriteBase
		policy.RewriteTarget = rule.RewriteTarget
		policy.RewritePrefixMatch = rule.RewritePrefixMatch
		policy.RewriteReplacePrefix = rule.RewriteReplacePrefix
		policy.EnableCORS = rule.EnableCORS
		policy.CORSAllowHeaders = rule.CORSAllowHeaders
		policy.CORSAllowOrigin = rule.CORSAllowOrigin
		policy.BackendProtocol = rule.BackendProtocol

		policy.RedirectScheme = rule.RedirectScheme
		policy.RedirectHost = rule.RedirectHost
		policy.RedirectPort = rule.RedirectPort
		policy.RedirectURL = rule.RedirectURL
		policy.RedirectPrefixMatch = rule.RedirectPrefixMatch
		policy.RedirectReplacePrefix = rule.RedirectReplacePrefix
		policy.RedirectCode = rule.RedirectCode

		policy.VHost = rule.VHost
		policy.Config = rule.Config
		policy.Source = rule.Source
		ngxPolicy.Http.Tcp[ft.Port] = append(ngxPolicy.Http.Tcp[ft.Port], &policy)
	}

	// set default rule if ft have default backend.
	if ft.BackendGroup != nil && ft.BackendGroup.Backends != nil {
		defaultPolicy := Policy{}
		defaultPolicy.Rule = ft.FtName
		defaultPolicy.Priority = -1     // default rule should have the minimum priority
		defaultPolicy.RawPriority = 999 // minimum number means higher priority
		defaultPolicy.Subsystem = SubsystemHTTP
		defaultPolicy.InternalDSL = []interface{}{[]string{"STARTS_WITH", "URL", "/"}} // [[START_WITH URL /]]
		defaultPolicy.BackendProtocol = ft.BackendProtocol
		defaultPolicy.Upstream = ft.BackendGroup.Name
		ngxPolicy.Http.Tcp[ft.Port] = append(ngxPolicy.Http.Tcp[ft.Port], &defaultPolicy)
	}
	sort.Sort(ngxPolicy.Http.Tcp[ft.Port]) // IMPORTANT sort to make sure priority work.
}

func (p *PolicyCli) initGRPCModeFt(ft *Frontend, ngxPolicy *NgxPolicy) {
	log := p.log
	if _, ok := ngxPolicy.Http.Tcp[ft.Port]; !ok {
		ngxPolicy.Http.Tcp[ft.Port] = Policies{}
	}

	for _, rule := range ft.Rules {
		if rule.DSLX == nil {
			log.V(3).Info("no matcher for rule, skip", "ruleID", rule.RuleID)
			continue
		}

		klog.V(3).Infof("Rule is %v", rule)
		policy := Policy{}
		policy.Subsystem = SubsystemHTTP
		policy.Rule = rule.RuleID
		internalDSL, err := utils.DSLX2Internal(rule.DSLX)
		if err != nil {
			log.Error(err, "convert dslx to internal failed", "ruleID", rule.RuleID)
			continue
		}
		policy.InternalDSL = internalDSL
		policy.RawPriority = rule.GetRawPriority()
		policy.InternalDSLLen = utils.InternalDSLLen(internalDSL)
		// policy-gen 设置 rule 的 upstream
		policy.Upstream = rule.BackendGroup.Name // IMPORTANT
		// for rewrite
		policy.URL = rule.URL
		policy.BackendProtocol = rule.BackendProtocol
		policy.Config = rule.Config
		ngxPolicy.Http.Tcp[ft.Port] = append(ngxPolicy.Http.Tcp[ft.Port], &policy)
	}

	// set default rule if ft have default backend.
	if ft.BackendGroup != nil && ft.BackendGroup.Backends != nil {
		defaultPolicy := Policy{}
		defaultPolicy.Rule = ft.FtName
		defaultPolicy.RawPriority = 999 // default backend has the lowest priority
		defaultPolicy.Subsystem = SubsystemHTTP
		defaultPolicy.InternalDSL = []interface{}{[]string{"STARTS_WITH", "URL", "/"}} // [[START_WITH URL /]]
		defaultPolicy.BackendProtocol = ft.BackendProtocol
		defaultPolicy.Upstream = ft.BackendGroup.Name
		ngxPolicy.Http.Tcp[ft.Port] = append(ngxPolicy.Http.Tcp[ft.Port], &defaultPolicy)
	}
	sort.Sort(ngxPolicy.Http.Tcp[ft.Port])
}

func (p *PolicyCli) FillUpBackends(cAlb *LoadBalancer) error {
	services := p.loadServices(cAlb)
	backendMap := make(map[string][]*driver.Backend)
	log := p.log
	for key, svc := range services {
		if svc == nil {
			continue
		}
		backendMap[key] = svc.Backends
	}

	for _, ft := range cAlb.Frontends {
		var rules RuleList
		protocol := m.FtProtocolToServiceProtocol(ft.Protocol)
		for _, rule := range ft.Rules {
			if len(rule.Services) == 0 {
				log.Info("no active service", "ruleID", rule.RuleID)
			}
			rule.BackendGroup = &BackendGroup{
				Name:                     rule.RuleID,
				Mode:                     FtProtocolToBackendMode(ft.Protocol),
				SessionAffinityPolicy:    rule.SessionAffinityPolicy,
				SessionAffinityAttribute: rule.SessionAffinityAttr,
			}
			rule.BackendGroup.Backends = generateBackend(backendMap, rule.Services, protocol)
			// if backend app protocol is https. use https.
			if rule.BackendProtocol == "$http_backend_protocol" {
				rule.BackendProtocol = "http"
				for _, b := range rule.BackendGroup.Backends {
					if b.AppProtocol != nil && strings.ToLower(*b.AppProtocol) == "https" {
						rule.BackendProtocol = "https"
					}
				}
			}
			rules = append(rules, rule)
		}
		if len(rules) > 0 {
			ft.Rules = rules
		} else {
			ft.Rules = RuleList{}
		}

		if len(ft.Services) == 0 {
			log.V(1).Info("frontend has no default service", "ft", ft.String())
		}
		if len(ft.Services) != 0 {
			// set ft default services group.
			ft.BackendGroup.Backends = generateBackend(backendMap, ft.Services, protocol)
			ft.BackendGroup.Mode = FtProtocolToBackendMode(ft.Protocol)
		}
	}
	return nil
}

func (p *PolicyCli) loadServices(alb *LoadBalancer) map[string]*driver.Service {
	kd := p.drv
	svcMap := make(map[string]*driver.Service)

	getServiceWithCache := func(svc *BackendService, protocol corev1.Protocol, svcMap map[string]*driver.Service) error {
		svcKey := generateServiceKey(svc.ServiceNs, svc.ServiceName, protocol, svc.ServicePort)
		if _, ok := svcMap[svcKey]; !ok {
			service, err := kd.GetServiceByName(svc.ServiceNs, svc.ServiceName, svc.ServicePort, protocol)
			if service != nil {
				svcMap[svcKey] = service
			}
			return err
		}
		return nil
	}

	for _, ft := range alb.Frontends {
		protocol := m.FtProtocolToServiceProtocol(ft.Protocol)
		for _, svc := range ft.Services {
			err := getServiceWithCache(svc, protocol, svcMap)
			if err != nil {
				klog.Errorf("get backends for ft fail svc %s/%s port %d protocol %s ft %s err %v", svc.ServiceName, svc.ServiceNs, svc.ServicePort, protocol, ft.FtName, err)
			}
		}

		for _, rule := range ft.Rules {
			if rule.AllowNoAddr() {
				continue
			}
			for _, svc := range rule.Services {
				err := getServiceWithCache(svc, protocol, svcMap)
				if err != nil {
					klog.Errorf("get backends for rule fail svc %s/%s %d protocol %s rule %s err %v", svc.ServiceName, svc.ServiceNs, svc.ServicePort, protocol, rule.RuleID, err)
				}
			}
		}
	}
	return svcMap
}

func generateServiceKey(ns string, name string, protocol corev1.Protocol, svcPort int) string {
	key := fmt.Sprintf("%s-%s-%s-%d", ns, name, protocol, svcPort)
	return strings.ToLower(key)
}

// 找到 service 对应的后端
func generateBackend(backendMap map[string][]*driver.Backend, services []*BackendService, protocol corev1.Protocol) Backends {
	totalWeight := 0
	for _, svc := range services {
		if svc.Weight > 100 {
			svc.Weight = 100
		}
		if svc.Weight < 0 {
			svc.Weight = 0
		}
		totalWeight += svc.Weight
	}
	if totalWeight == 0 {
		// all service has zero weight
		totalWeight = 100
	}
	bes := []*Backend{}
	for _, svc := range services {
		name := generateServiceKey(svc.ServiceNs, svc.ServiceName, protocol, svc.ServicePort)
		backends, ok := backendMap[name]
		// some rule such as redirect ingress will use a fake service.
		if !ok || len(backends) == 0 {
			continue
		}
		// 100 is the max weigh in SLB
		weight := int(math.Floor(float64(svc.Weight*100)/float64(totalWeight*len(backends)) + 0.5))
		if weight == 0 && svc.Weight != 0 {
			weight = 1
		}
		for _, be := range backends {
			port := be.Port
			if port == 0 {
				klog.Warningf("invalid backend port 0 svc: %+v", svc)
				continue
			}
			bes = append(bes,
				&Backend{
					Address:           be.IP,
					Pod:               be.Pod,
					Port:              port,
					Weight:            weight,
					Protocol:          be.Protocol,
					AppProtocol:       be.AppProtocol,
					FromOtherClusters: be.FromOtherClusters,
				})
		}
	}
	sortedBackends := Backends(bes)
	sort.Sort(sortedBackends)
	return sortedBackends
}

func (p *PolicyCli) setMetricsPortCert(cert map[string]types.Certificate) {
	port := p.opt.MetricsPort
	cert[fmt.Sprintf("%d", port)] = genMetricsCert()
}

var (
	metricsCert types.Certificate
	once        sync.Once
)

func genMetricsCert() types.Certificate {
	once.Do(func() {
		cert, key, _ := certutil.GenerateSelfSignedCertKey("localhost", []net.IP{}, []string{})
		metricsCert = types.Certificate{
			Cert: string(cert),
			Key:  string(key),
		}
	})
	return metricsCert
}
