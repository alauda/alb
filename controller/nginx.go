package controller

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"text/template"
	"time"

	m "alauda.io/alb2/modules"
	albv1 "alauda.io/alb2/pkg/apis/alauda/v1"
	"github.com/thoas/go-funk"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"context"

	"alauda.io/alb2/config"
	. "alauda.io/alb2/controller/types"
	"alauda.io/alb2/driver"
	g "alauda.io/alb2/gateway"
	gateway "alauda.io/alb2/gateway/nginx"
	"alauda.io/alb2/utils"
	. "alauda.io/alb2/utils/log"
	"k8s.io/klog/v2"
)

type NginxController struct {
	BackendType   string
	TemplatePath  string
	NewConfigPath string // in fact, the updated nginx.conf
	OldConfigPath string // in fact, the current nginx.conf
	NewPolicyPath string
	BinaryPath    string
	Driver        *driver.KubernetesDriver
}

type Policy struct {
	Rule             string        `json:"rule"` // the name of rule, corresponding with k8s rule cr
	Config           *RuleConfig   `json:"config,omitempty"`
	InternalDSL      []interface{} `json:"internal_dsl"` // dsl determine whether a request match this rule, same as rule.spec.dlsx
	InternalDSLLen   int           `json:"-"`            // the len of jsonstringify internal dsl, used to sort policy
	Upstream         string        `json:"upstream"`     // name in backend group
	URL              string        `json:"url"`
	RewriteBase      string        `json:"rewrite_base"`
	RewriteTarget    string        `json:"rewrite_target"`
	Priority         int           `json:"complexity_priority"` // priority calculated by the complex of dslx, used to sort policy after user_priority
	RawPriority      int           `json:"user_priority"`       // priority set by user, used to sort policy
	Subsystem        string        `json:"subsystem"`
	EnableCORS       bool          `json:"enable_cors"`
	CORSAllowHeaders string        `json:"cors_allow_headers"`
	CORSAllowOrigin  string        `json:"cors_allow_origin"`
	BackendProtocol  string        `json:"backend_protocol"`
	RedirectScheme   *string       `json:"redirect_scheme,omitempty"`
	RedirectHost     *string       `json:"redirect_host,omitempty"`
	RedirectPort     *int          `json:"redirect_port,omitempty"`
	RedirectURL      string        `json:"redirect_url"`
	RedirectCode     int           `json:"redirect_code"`
	VHost            string        `json:"vhost"`
}

type NgxPolicy struct {
	CertificateMap map[string]Certificate `json:"certificate_map"`
	Http           HttpPolicy             `json:"http"`
	Stream         StreamPolicy           `json:"stream"`
	BackendGroup   []*BackendGroup        `json:"backend_group"`
}

type HttpPolicy struct {
	Tcp map[albv1.PortNumber]Policies `json:"tcp"`
}

type StreamPolicy struct {
	Tcp map[albv1.PortNumber]Policies `json:"tcp"`
	Udp map[albv1.PortNumber]Policies `json:"udp"`
}

type Policies []*Policy

func (p Policies) Len() int { return len(p) }

func (p Policies) Less(i, j int) bool {
	// raw priority is set by user it should be [1,10]. the smaller the number, the higher the ranking
	if p[i].RawPriority != p[j].RawPriority {
		return p[i].RawPriority < p[j].RawPriority
	}
	// priority is calculated by the "complex" of this policy. the bigger the number, the higher the ranking
	if p[i].Priority != p[j].Priority {
		return p[i].Priority > p[j].Priority
	}
	if p[i].InternalDSLLen != p[j].InternalDSLLen {
		return p[i].InternalDSLLen > p[j].InternalDSLLen
	}
	return p[i].Rule < p[j].Rule
}

func (p Policies) Swap(i, j int) { p[i], p[j] = p[j], p[i] }

func NewNginxController(kd *driver.KubernetesDriver) *NginxController {
	return &NginxController{
		TemplatePath:  config.Get("NGINX_TEMPLATE_PATH"),
		NewConfigPath: config.Get("NEW_CONFIG_PATH"),
		OldConfigPath: config.Get("OLD_CONFIG_PATH"),
		NewPolicyPath: config.Get("NEW_POLICY_PATH"),
		BackendType:   kd.GetType(),
		BinaryPath:    config.Get("NGINX_BIN_PATH"),
		Driver:        kd}
}

func (nc *NginxController) GetLoadBalancerType() string {
	return "Nginx"
}

func (nc *NginxController) GenerateConf() error {
	nginxConfig, ngxPolicies, err := nc.GenerateNginxConfigAndPolicy()
	if err != nil {
		return err
	}
	nc.WriteConfig(nginxConfig, ngxPolicies)

	if err != nil {
		return err
	}
	return nil
}

func (nc *NginxController) GetLBConfig() (*LoadBalancer, error) {
	ns := config.Get("NAMESPACE")
	name := config.Get("NAME")
	lb, err := nc.GetLBConfigFromAlb(ns, name)
	if err != nil {
		return nil, err
	}
	detectAndMaskConflictPort(lb, nc.Driver)
	migratePortProject(lb, nc.Driver)

	if !config.GetBool("ENABLE_GATEWAY") {
		return lb, nil
	}
	if config.GetBool("DISABLE_GATEWAY_NGINX_CONFIG") {
		return lb, nil
	}
	log := L().WithName(g.ALB_GATEWAY_NGINX)
	lbFromGateway, err := gateway.GetLBConfig(context.Background(), nc.Driver, name)
	if err != nil {
		log.Error(err, "get lb from gateway fail", "class", name)
		return nil, err
	}
	// no gateway found
	if lbFromGateway == nil {
		log.Info("no gateway found ignore")
		return lb, nil
	}

	log.V(2).Info("lb config from gateway ", "lbconfig", lbFromGateway)
	lb, err = nc.MergeLBConfig(lb, lbFromGateway)
	if err != nil {
		log.Error(err, "merge config fail ")
		return nil, err
	}
	return lb, err
}

func (nc *NginxController) GenerateNginxConfigAndPolicy() (nginxTemplateConfig NginxTemplateConfig, nginxPolicy NgxPolicy, err error) {
	alb, err := nc.GetLBConfig()
	if err != nil {
		return NginxTemplateConfig{}, NgxPolicy{}, err
	}
	err = nc.fillUpBackends(alb)
	if err != nil {
		return NginxTemplateConfig{}, NgxPolicy{}, err
	}

	if len(alb.Frontends) == 0 {
		ns := config.Get("NAMESPACE")
		name := config.Get("NAME")
		klog.Infof("No service bind to this nginx now %v/%v", ns, name)
	}

	nginxPolicy = nc.generateAlbPolicy(alb)
	phase := config.Get("PHASE")
	if phase != m.PhaseTerminating {
		phase = m.PhaseRunning
	}
	cfg, err := GenerateNginxTemplateConfig(alb, phase, newNginxParam())
	if err != nil {
		return NginxTemplateConfig{}, NgxPolicy{}, fmt.Errorf("generate nginx.conf fail %v", err)
	}
	return *cfg, nginxPolicy, nil
}

// GetLBConfigFromAlb get lb config from alb/ft/rule.
func (nc *NginxController) GetLBConfigFromAlb(ns string, name string) (*LoadBalancer, error) {
	// mAlb LoadBalancer struct from modules package.
	// cAlb LoadBalancer struct from controller package.
	kd := nc.Driver
	mAlb, err := kd.LoadALBbyName(ns, name)
	klog.Infof("get alb %+v", mAlb)
	if err != nil {
		klog.Error("load mAlb fail", err)
		return nil, err
	}

	cAlb := &LoadBalancer{
		Name:      mAlb.Name,
		Address:   mAlb.Spec.Address,
		Frontends: []*Frontend{},
		TweakHash: mAlb.TweakHash,
		Labels:    mAlb.Labels,
	}

	// mft frontend struct from modules package.
	for _, mft := range mAlb.Frontends {
		ft := &Frontend{
			FtName:          mft.Name,
			AlbName:         mAlb.Name,
			Port:            mft.Port,
			Protocol:        mft.Protocol,
			Rules:           RuleList{},
			CertificateName: mft.CertificateName,
			BackendProtocol: mft.BackendProtocol,
			Labels:          mft.Lables,
		}
		if !ft.IsValidProtocol() {
			klog.Errorf("frontend %s %s has no valid protocol", ft.FtName, ft.Protocol)
			ft.Protocol = albv1.FtProtocolTCP
		}

		if ft.Port <= 0 {
			klog.Errorf("frontend %s has an invalid port %d", ft.FtName, ft.Port)
			continue
		}

		// migrate rule
		for _, arl := range mft.Rules {
			ruleConfig := RuleConfigFromRuleAnnotation(arl.Name, arl.Annotations)
			rule := &Rule{
				Config:           ruleConfig,
				RuleID:           arl.Name,
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

		if mft.ServiceGroup != nil {
			ft.Services = []*BackendService{}
			ft.BackendGroup = &BackendGroup{
				Name:                     ft.String(),
				SessionAffinityAttribute: mft.ServiceGroup.SessionAffinityAttribute,
				SessionAffinityPolicy:    mft.ServiceGroup.SessionAffinityPolicy,
			}

			for _, svc := range mft.ServiceGroup.Services {
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

func (nc *NginxController) fillUpBackends(cAlb *LoadBalancer) error {

	services := nc.LoadServices(cAlb)

	backendMap := make(map[string][]*driver.Backend)
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
				klog.Warningf("rule %s has no active service.", rule.RuleID)
			}
			rule.BackendGroup = &BackendGroup{
				Name:                     rule.RuleID,
				Mode:                     FtProtocolToBackendMode(ft.Protocol),
				SessionAffinityPolicy:    rule.SessionAffinityPolicy,
				SessionAffinityAttribute: rule.SessionAffinityAttr,
			}
			rule.BackendGroup.Backends = generateBackend(backendMap, rule.Services, protocol)
			// NOTE: if backend app protocol is https. use https.
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
			klog.Warningf("frontend %s has no default service.", ft.String())
		}
		if len(ft.Services) != 0 {
			// set ft default services group.
			ft.BackendGroup.Backends = generateBackend(backendMap, ft.Services, protocol)
			ft.BackendGroup.Mode = FtProtocolToBackendMode(ft.Protocol)
		}
	}
	klog.V(3).Infof("Get alb : %s", cAlb.String())
	return nil
}

func (nc *NginxController) initStreamModeFt(ft *Frontend, ngxPolicy *NgxPolicy) {
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

func (nc *NginxController) initHttpModeFt(ft *Frontend, ngxPolicy *NgxPolicy) {
	if _, ok := ngxPolicy.Http.Tcp[ft.Port]; !ok {
		ngxPolicy.Http.Tcp[ft.Port] = Policies{}
	}

	for _, rule := range ft.Rules {
		if rule.DSLX == nil {
			klog.Warningf("rule %s has no matcher, skip", rule.RuleID)
			continue
		}

		klog.V(3).Infof("Rule is %v", rule)
		policy := Policy{}
		policy.Subsystem = SubsystemHTTP
		policy.Rule = rule.RuleID
		internalDSL, err := utils.DSLX2Internal(rule.DSLX)
		if err != nil {
			klog.Error("convert dslx to internal failed", err)
			continue
		}
		policy.InternalDSL = internalDSL

		policy.Priority = rule.GetPriority()
		policy.RawPriority = rule.GetRawPriority()
		policy.InternalDSLLen = utils.InternalDSLLen(internalDSL)
		// policy-gen 设置rule的upstream
		policy.Upstream = rule.BackendGroup.Name // IMPORTANT
		// for rewrite
		policy.URL = rule.URL
		policy.RewriteBase = rule.RewriteBase
		policy.RewriteTarget = rule.RewriteTarget
		policy.EnableCORS = rule.EnableCORS
		policy.CORSAllowHeaders = rule.CORSAllowHeaders
		policy.CORSAllowOrigin = rule.CORSAllowOrigin
		policy.BackendProtocol = rule.BackendProtocol

		policy.RedirectScheme = rule.RedirectScheme
		policy.RedirectHost = rule.RedirectHost
		policy.RedirectPort = rule.RedirectPort
		policy.RedirectURL = rule.RedirectURL
		policy.RedirectCode = rule.RedirectCode

		policy.VHost = rule.VHost
		policy.Config = rule.Config
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

func (nc *NginxController) initGRPCModeFt(ft *Frontend, ngxPolicy *NgxPolicy) {
	if _, ok := ngxPolicy.Http.Tcp[ft.Port]; !ok {
		ngxPolicy.Http.Tcp[ft.Port] = Policies{}
	}

	for _, rule := range ft.Rules {
		if rule.DSLX == nil {
			klog.Warningf("rule %s has no matcher, skip", rule.RuleID)
			continue
		}

		klog.V(3).Infof("Rule is %v", rule)
		policy := Policy{}
		policy.Subsystem = SubsystemHTTP
		policy.Rule = rule.RuleID
		internalDSL, err := utils.DSLX2Internal(rule.DSLX)
		if err != nil {
			klog.Error("convert dslx to internal failed", err)
			continue
		}
		policy.InternalDSL = internalDSL
		policy.RawPriority = rule.GetRawPriority()
		policy.InternalDSLLen = utils.InternalDSLLen(internalDSL)
		// policy-gen 设置rule的upstream
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

// fetch cert and backend info that lb config neeed, constructs a "dynamic config" used by openresty.
func (nc *NginxController) generateAlbPolicy(alb *LoadBalancer) NgxPolicy {
	certificateMap := getCertMap(alb, nc.Driver)
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
			nc.initStreamModeFt(ft, &ngxPolicy)
		}

		if ft.IsHttpMode() {
			nc.initHttpModeFt(ft, &ngxPolicy)
		}

		if ft.IsGRPCMode() {
			nc.initGRPCModeFt(ft, &ngxPolicy)
		}
	}

	return ngxPolicy
}

func detectAndMaskConflictPort(alb *LoadBalancer, driver *driver.KubernetesDriver) {
	enablePortProbe := config.Get("ENABLE_PORTPROBE") == "true"
	if !enablePortProbe {
		return
	}
	var listenTCPPorts []int
	listenTCPPorts, err := utils.GetListenTCPPorts()
	if err != nil {
		klog.Error(err)
	}
	klog.V(2).Info("finish port probe, listen tcp ports: ", listenTCPPorts)

	for _, ft := range alb.Frontends {
		conflict := false
		if ft.IsTcpBaseProtocol() {
			for _, port := range listenTCPPorts {
				if int(ft.Port) == port {
					conflict = true
					ft.Conflict = true
					klog.Errorf("skip port: %d has conflict", ft.Port)
					break
				}
			}
			if err := driver.UpdateFrontendStatus(ft.FtName, conflict); err != nil {
				klog.Error(err)
			}
		}
	}
}

func migratePortProject(alb *LoadBalancer, driver *driver.KubernetesDriver) {
	var portInfo map[string][]string
	if GetAlbRoleType(alb.Labels) != RolePort {
		return
	}
	portInfo, err := getPortInfo(driver)
	if err != nil {
		klog.Errorf("get port project info failed, %v", err)
		return
	}
	for _, ft := range alb.Frontends {
		if ft.Conflict {
			continue
		}
		if GetAlbRoleType(alb.Labels) == RolePort && portInfo != nil {
			// current projects
			portProjects := GetOwnProjects(ft.FtName, ft.Labels)
			// desired projects
			desiredPortProjects, err := getPortProject(int(ft.Port), portInfo)
			if err != nil {
				klog.Errorf("get port %d desired projects failed, %v", ft.Port, err)
				return
			}
			if diff := funk.Subtract(portProjects, desiredPortProjects); diff != nil {
				// diff need update
				payload := generatePatchPortProjectPayload(ft.Labels, desiredPortProjects)
				if _, err := driver.ALBClient.CrdV1().Frontends(config.Get("NAMESPACE")).Patch(context.TODO(), ft.FtName, types.JSONPatchType, payload, metav1.PatchOptions{}); err != nil {
					klog.Errorf("patch port %s project failed, %v", ft.FtName, err)
				}
			}
		}
	}
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

func (nc *NginxController) LoadServices(alb *LoadBalancer) map[string]*driver.Service {
	kd := nc.Driver
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

func (nc *NginxController) WriteConfig(nginxTemplateConfig NginxTemplateConfig, ngxPolicies NgxPolicy) error {
	policyBytes, err := json.MarshalIndent(ngxPolicies, "", "\t")
	if err != nil {
		klog.Error()
		return err
	}
	configWriter, err := os.Create(nc.NewConfigPath)
	if err != nil {
		klog.Errorf("Failed to create new config file %s", err.Error())
		return err
	}
	defer configWriter.Close()
	policyWriter, err := os.Create(nc.NewPolicyPath)
	if err != nil {
		klog.Errorf("Failed to create new policy file %s", err.Error())
		return err
	}
	err = policyWriter.Chmod(0644)
	if err != nil {
		klog.Errorf("Failed to add read permission of policy.new for nginx user %s", err.Error())
		return err
	}
	defer policyWriter.Close()

	t, err := template.New("nginx.tmpl").ParseFiles(nc.TemplatePath)
	if err != nil {
		klog.Errorf("Failed to parse template %s", err.Error())
		return err
	}
	if _, err := policyWriter.Write(policyBytes); err != nil {
		klog.Errorf("Write policy file failed %s", err.Error())
		return err
	}
	policyWriter.Sync()

	err = t.Execute(configWriter, nginxTemplateConfig)
	if err != nil {
		klog.Error(err)
		return err
	}
	if err := configWriter.Sync(); err != nil {
		klog.Error(err)
		return err
	}
	return nil
}

func (nc *NginxController) ReloadLoadBalancer() error {
	StatusFileParentPath := config.Get("STATUSFILE_PARENTPATH")
	var err error
	defer func() {
		if err != nil {
			setLastReloadStatus(FAILED, StatusFileParentPath)
		} else {
			setLastReloadStatus(SUCCESS, StatusFileParentPath)
		}
	}()

	configChanged := !sameFiles(nc.NewConfigPath, nc.OldConfigPath)

	// No change, Nginx running, skip
	if !configChanged && getLastReloadStatus(StatusFileParentPath) == SUCCESS {
		klog.Info("Config not changed and last reload success")
		return nil
	}

	// Update config and policy files
	if configChanged {
		diffOutput, _ := exec.Command("diff", "-u", nc.OldConfigPath, nc.NewConfigPath).CombinedOutput()
		klog.Infof("NGINX configuration diff\n")
		klog.Infof("%v\n", string(diffOutput))

		klog.Info("Start to change config.")
		err = os.Rename(nc.NewConfigPath, nc.OldConfigPath)
		if err != nil {
			klog.Errorf("failed to replace config: %s", err.Error())
			return err
		}
	}

	if config.GetBool("E2E_TEST_CONTROLLER_ONLY") {
		klog.Info("test mode, do not touch nginx")
		return nil
	}

	// nginx process runs in an independent container, guaranteed by kubernetes
	nginxPid, err := GetProcessId()
	if nginxPid == "" {
		return err
	}
	nginxPid = strings.Trim(nginxPid, "\n ")

	if configChanged || getLastReloadStatus(StatusFileParentPath) == FAILED {
		err = nc.reload(nginxPid)
	} else {
		klog.V(3).Info("no need to manipulate nginx")
	}

	return err
}

func (nc *NginxController) reload(nginxPid string) error {
	klog.Info("Send HUP signal to reload nginx")
	output, err := exec.Command("kill", "-HUP", nginxPid).CombinedOutput()
	if err != nil {
		klog.Errorf("reload nginx failed: %s %v", output, err)
	}
	return err
}

func (nc *NginxController) GC() error {
	gcOpt := GCOptions{
		GCServiceRule: config.Get("ENABLE_GC") == "true",
		GCAppRule:     config.Get("ENABLE_GC_APP_RULE") == "true",
	}
	if !gcOpt.GCServiceRule && !gcOpt.GCAppRule {
		return nil
	}
	start := time.Now()
	klog.Info("begin gc rule")
	defer klog.Infof("end gc rule, spend time %s", time.Since(start))
	return GCRule(nc.Driver, gcOpt)
}

func (nc *NginxController) MergeLBConfig(alb *LoadBalancer, gateway *LoadBalancer) (*LoadBalancer, error) {
	ftInAlb := make(map[string]string)
	for _, ft := range alb.Frontends {
		key := fmt.Sprintf("%v/%v", ft.Protocol, ft.Port)
		ftInAlb[key] = ft.FtName
	}
	for _, ft := range gateway.Frontends {
		key := fmt.Sprintf("%v/%v", ft.Protocol, ft.Port)
		albFtName, find := ftInAlb[key]
		if find {
			klog.Warningf("merge-gateway: find conflict port %v between gateway %v and alb %v ignore this gateway ft", ft.Port, ft.FtName, albFtName)
			continue
		}
		alb.Frontends = append(alb.Frontends, ft)
	}

	return alb, nil
}
