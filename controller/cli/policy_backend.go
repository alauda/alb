package cli

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	m "alauda.io/alb2/controller/modules"
	. "alauda.io/alb2/controller/types"
	"alauda.io/alb2/driver"
	pm "alauda.io/alb2/pkg/utils/metrics"
	"github.com/thoas/go-funk"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

func (p *PolicyCli) FillUpBackends(cAlb *LoadBalancer) error {
	s := time.Now()
	defer func() {
		pm.Write("fill-backends", float64(time.Since(s).Milliseconds()))
	}()

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
					Svc:               svc.ServiceName,
					Ns:                svc.ServiceNs,
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

func pickAllBackendGroup(alb *LoadBalancer) BackendGroups {
	s := time.Now()
	defer func() {
		pm.Write("pick-backends", float64(time.Since(s).Milliseconds()))
	}()
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
