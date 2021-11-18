package controller

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"text/template"
	"time"

	"alauda.io/alb2/config"
	"alauda.io/alb2/driver"
	"alauda.io/alb2/utils"
	"k8s.io/klog"
)

const (
	DEFAULT_RULE = "(STARTS_WITH URL /)"
)

type NginxController struct {
	BackendType   string
	TemplatePath  string
	NewConfigPath string
	OldConfigPath string
	NewPolicyPath string
	BinaryPath    string
	Driver        *driver.KubernetesDriver
}

type Policy struct {
	Rule             string        `json:"rule"`
	Config           *RuleConfig    `json:"config,omitempty"`
	DSL              string        `json:"dsl"`
	InternalDSL      []interface{} `json:"internal_dsl"`
	Upstream         string        `json:"upstream"`
	URL              string        `json:"url"`
	RewriteBase      string        `json:"rewrite_base"`
	RewriteTarget    string        `json:"rewrite_target"`
	Priority         int           `json:"-"`
	RawPriority      int           `json:"-"`
	Subsystem        string        `json:"subsystem"`
	EnableCORS       bool          `json:"enable_cors"`
	CORSAllowHeaders string        `json:"cors_allow_headers"`
	CORSAllowOrigin  string        `json:"cors_allow_origin"`
	BackendProtocol  string        `json:"backend_protocol"`
	RedirectURL      string        `json:"redirect_url"`
	RedirectCode     int           `json:"redirect_code"`
	VHost            string        `json:"vhost"`
}

type NgxPolicy struct {
	CertificateMap map[string]Certificate `json:"certificate_map"`
	PortMap        map[int]Policies       `json:"port_map"`
	BackendGroup   []*BackendGroup        `json:"backend_group"`
}

type Policies []*Policy

func (p Policies) Len() int { return len(p) }

func (p Policies) Less(i, j int) bool {
	if p[i].RawPriority > p[j].RawPriority {
		return false
	}
	if p[i].RawPriority < p[j].RawPriority {
		return true
	}
	return p[i].Priority > p[j].Priority
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

func (nc *NginxController) generateNginxConfigAndAlbPolicy(loadbalancer *LoadBalancer) (Config, NgxPolicy) {
	config := generateConfig(loadbalancer, nc.Driver)
	ngxPolicy := NgxPolicy{
		CertificateMap: config.CertificateMap,
		PortMap:        make(map[int]Policies),
		BackendGroup:   config.BackendGroup,
	}
	for port, frontend := range config.Frontends {
		klog.V(3).Infof("Frontend is %+v", frontend)
		if _, ok := ngxPolicy.PortMap[port]; !ok {
			ngxPolicy.PortMap[port] = Policies{}
		}
		for _, rule := range frontend.Rules {
			// for compatible
			rule.FillupDSL()
			if rule.DSL == "" && rule.DSLX == nil {
				klog.Warningf("rule %s has no matcher, skip", rule.RuleID)
				continue
			}

			klog.V(3).Infof("Rule is %v", rule)
			policy := Policy{}
			if frontend.Protocol == ProtocolHTTP || frontend.Protocol == ProtocolHTTPS {
				policy.Subsystem = SubsystemHTTP
			} else {
				//ProtocolTCP
				policy.Subsystem = SubsystemStream
			}
			policy.Rule = rule.RuleID
			policy.DSL = rule.DSL
			if rule.DSLX == nil {
				dslx, err := utils.DSL2DSLX(rule.DSL)
				if err != nil {
					klog.Errorf("convert dsl to dslx failed rule %v dsl %v err %v", rule.RuleID, rule.DSL, err)
				} else {
					rule.DSLX = dslx
				}
			}
			if rule.DSLX != nil {
				internalDSL, err := utils.DSLX2Internal(rule.DSLX)
				if err != nil {
					klog.Error("convert dslx to internal failed", err)
				} else {
					policy.InternalDSL = internalDSL
				}
			}
			policy.Priority = rule.GetPriority()
			policy.RawPriority = rule.GetRawPriority()
			policy.Upstream = rule.BackendGroup.Name
			// for rewrite
			policy.URL = rule.URL
			policy.RewriteBase = rule.RewriteBase
			policy.RewriteTarget = rule.RewriteTarget
			policy.EnableCORS = rule.EnableCORS
			policy.CORSAllowHeaders = rule.CORSAllowHeaders
			policy.CORSAllowOrigin = rule.CORSAllowOrigin
			policy.BackendProtocol = rule.BackendProtocol
			policy.RedirectURL = rule.RedirectURL
			policy.RedirectCode = rule.RedirectCode
			policy.VHost = rule.VHost
			policy.Config = rule.Config
			ngxPolicy.PortMap[port] = append(ngxPolicy.PortMap[port], &policy)
		}

		// set default rule if exists
		defaultPolicy := Policy{}
		// default rule should have the minimum priority
		defaultPolicy.Priority = -1
		defaultPolicy.RawPriority = 999 // minimum number means higher priority
		if frontend.Protocol == ProtocolHTTP || frontend.Protocol == ProtocolHTTPS {
			defaultPolicy.Subsystem = SubsystemHTTP
		} else {
			//ProtocolTCP
			defaultPolicy.Subsystem = SubsystemStream
		}
		if frontend.Protocol != ProtocolTCP {
			defaultPolicy.Rule = frontend.RawName
			defaultPolicy.DSL = DEFAULT_RULE
			defaultPolicy.BackendProtocol = frontend.BackendProtocol
		}
		if frontend.BackendGroup != nil && frontend.BackendGroup.Backends != nil {
			defaultPolicy.Upstream = frontend.BackendGroup.Name
			ngxPolicy.PortMap[port] = append(ngxPolicy.PortMap[port], &defaultPolicy)
		}
		sort.Sort(ngxPolicy.PortMap[port])
	}
	return config, ngxPolicy
}

var loadBalancersCache []byte
var nextFetchTime time.Time
var infoLock sync.Mutex

//FetchLoadBalancersInfo return loadbalancer info from cache, mirana2 or apiserver
func (nc *NginxController) FetchLoadBalancersInfo() ([]*LoadBalancer, error) {
	infoLock.Lock()
	defer infoLock.Unlock()
	if time.Now().Before(nextFetchTime) && loadBalancersCache != nil {
		var lbs []*LoadBalancer
		//make sure always return a copy of loadbalaners
		err := json.Unmarshal(loadBalancersCache, &lbs)
		if err != nil {
			// should never happen
			klog.Error(err)
			panic(err)
		}
		return lbs, nil
	}

	alb, err := nc.Driver.LoadALBbyName(config.Get("NAMESPACE"), config.Get("NAME"))
	if err != nil {
		klog.Error(err)
		return []*LoadBalancer{}, nil
	}

	lb, err := MergeNew(alb)
	if err != nil {
		klog.Error(err)
		return nil, err
	}
	var loadBalancers = []*LoadBalancer{
		lb,
	}

	interval := config.GetInt("INTERVAL")
	nextFetchTime = time.Now().Add(time.Duration(interval) * time.Second)
	loadBalancersCache, _ = json.Marshal(loadBalancers)
	klog.V(3).Infof("Get Loadbalancers: %s", string(loadBalancersCache))
	return loadBalancers, err
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

func (nc *NginxController) GenerateNginxConfigAndPolicy() (nginxTemplateConfig NginxTemplateConfig, nginxPolicy NgxPolicy, err error) {
	services, err := nc.Driver.ListService()
	if err != nil {
		return NginxTemplateConfig{}, NgxPolicy{}, err
	}
	loadbalancers, err := nc.FetchLoadBalancersInfo()
	if err != nil {
		return NginxTemplateConfig{}, NgxPolicy{}, err
	}

	merge(loadbalancers, services)

	if len(loadbalancers) == 0 {
		return NginxTemplateConfig{}, NgxPolicy{}, errors.New("no lb found")
	}
	if len(loadbalancers[0].Frontends) == 0 {
		klog.Info("No service bind to this nginx now")
	}

	nginxConfig, nginxPolicy := nc.generateNginxConfigAndAlbPolicy(loadbalancers[0])

	cfg, err := NewNginxTemplateConfigGenerator(nginxConfig).Generate()
	if err != nil {
		return NginxTemplateConfig{}, NgxPolicy{}, fmt.Errorf("generate nginx.conf fail %v", err)
	}
	return cfg, nginxPolicy, nil
}

func (nc *NginxController) WriteConfig(nginxTemplateConfig NginxTemplateConfig, ngxPolicies NgxPolicy) error {
	policyBytes, err := json.Marshal(ngxPolicies)
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
	if nc.BinaryPath == "" {
		// set it to empty for local test
		klog.Errorf("Nginx bin path is empty!!!")
		return nil
	}

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
