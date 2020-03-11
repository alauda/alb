package controller

import (
	"alauda.io/alb2/utils"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"text/template"

	"alauda.io/alb2/config"
	"alauda.io/alb2/driver"

	"github.com/golang/glog"
)

const DEFAULT_RULE = "(STARTS_WITH URL /)"

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
	Rule            string        `json:"rule"`
	DSL             string        `json:"dsl"`
	InternalDSL     []interface{} `json:"internal_dsl"`
	Upstream        string        `json:"upstream"`
	URL             string        `json:"url"`
	RewriteTarget   string        `json:"rewrite_target"`
	Priority        int           `json:"priority"`
	Subsystem       string        `json:"subsystem"`
	EnableCORS      bool          `json:"enable_cors"`
	BackendProtocol string        `json:"backend_protocol"`
	RedirectURL     string        `json:"redirect_url"`
	RedirectCode    int           `json:"redirect_code"`
}

type NgxPolicy struct {
	CertificateMap map[string]Certificate `json:"certificate_map"`
	PortMap        map[int]Policies       `json:"port_map"`
	BackendGroup   []*BackendGroup        `json:"backend_group"`
}

type Policies []*Policy

func (p Policies) Len() int           { return len(p) }
func (p Policies) Less(i, j int) bool { return p[i].Priority > p[j].Priority }
func (p Policies) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

func (nc *NginxController) GetLoadBalancerType() string {
	return "Nginx"
}

func (nc *NginxController) generateNginxConfig(loadbalancer *LoadBalancer) (Config, NgxPolicy) {
	config := generateConfig(loadbalancer, nc.Driver)
	ngxPolicy := NgxPolicy{
		CertificateMap: config.CertificateMap,
		PortMap:        make(map[int]Policies),
		BackendGroup:   config.BackendGroup,
	}
	for port, frontend := range config.Frontends {
		glog.V(3).Infof("Frontend is %+v", frontend)
		if _, ok := ngxPolicy.PortMap[port]; !ok {
			ngxPolicy.PortMap[port] = Policies{}
		}
		for _, rule := range frontend.Rules {
			if rule.BackendGroup == nil {
				continue
			}

			// for compatible
			if rule.DSL == "" && (rule.Domain != "" || rule.URL != "") {
				glog.Info("transfer rule to dsl")
				if rule.Domain != "" && rule.URL != "" {
					if strings.HasPrefix(rule.URL, "^") {
						rule.DSL = fmt.Sprintf("(AND (EQ HOST %s) (REGEX URL %s))", rule.Domain, rule.URL)
					} else {
						rule.DSL = fmt.Sprintf("(AND (EQ HOST %s) (STARTS_WITH URL %s))", rule.Domain, rule.URL)
					}
				} else {
					if rule.Domain != "" {
						rule.DSL = fmt.Sprintf("(EQ HOST %s)", rule.Domain)
					} else {
						if strings.HasPrefix(rule.URL, "^") {
							rule.DSL = fmt.Sprintf("(REGEX URL %s)", rule.URL)
						} else {
							rule.DSL = fmt.Sprintf("(STARTS_WITH URL %s)", rule.URL)
						}
					}
				}
				if rule.DSL != "" {
					dslx, err := utils.DSL2DSLX(rule.DSL)
					if err != nil {
						glog.Warning(err)
					} else {
						rule.DSLX = dslx
					}
				}
			}

			if rule.DSL == "" {
				continue
			}

			glog.V(3).Infof("Rule is %v", rule)
			policy := Policy{}
			if frontend.Protocol == ProtocolHTTP || frontend.Protocol == ProtocolHTTPS {
				policy.Subsystem = SubsystemHTTP
			} else if frontend.Protocol == ProtocolTCP {
				policy.Subsystem = SubsystemStream
			}
			policy.Rule = rule.RuleID
			policy.DSL = rule.DSL
			if rule.DSLX != nil {
				internalDSL, err := utils.DSLX2Internal(rule.DSLX)
				if err != nil {
					glog.Error("convert dslx to internal failed", err)
				} else {
					policy.InternalDSL = internalDSL
				}
			}
			if rule.Priority != 0 {
				policy.Priority = int(rule.Priority)
			} else {
				policy.Priority = len(rule.DSL)
			}
			policy.Upstream = rule.BackendGroup.Name
			// for rewrite
			policy.URL = rule.URL
			policy.RewriteTarget = rule.RewriteTarget
			policy.EnableCORS = rule.EnableCORS
			policy.BackendProtocol = rule.BackendProtocol
			policy.RedirectURL = rule.RedirectURL
			policy.RedirectCode = rule.RedirectCode
			ngxPolicy.PortMap[port] = append(ngxPolicy.PortMap[port], &policy)
		}

		// set default rule if exists
		if frontend.BackendGroup != nil && frontend.BackendGroup.Backends != nil {
			glog.V(3).Infof("Default rule is %v", frontend.BackendGroup)
			policy := Policy{}
			if frontend.Protocol == ProtocolHTTP || frontend.Protocol == ProtocolHTTPS {
				policy.Subsystem = SubsystemHTTP
			} else if frontend.Protocol == ProtocolTCP {
				policy.Subsystem = SubsystemStream
			}
			if frontend.Protocol != ProtocolTCP {
				policy.Rule = frontend.RawName
				policy.DSL = DEFAULT_RULE
			}
			policy.Upstream = frontend.BackendGroup.Name
			ngxPolicy.PortMap[port] = append(ngxPolicy.PortMap[port], &policy)
		}
		sort.Sort(ngxPolicy.PortMap[port])
	}
	return config, ngxPolicy
}

func (nc *NginxController) GenerateConf() error {
	services, err := nc.Driver.ListService()
	if err != nil {
		return err
	}
	loadbalancers, err := FetchLoadBalancersInfo()
	if err != nil {
		return err
	}

	merge(loadbalancers, services)

	if len(loadbalancers) == 0 {
		return errors.New("no lb found")
	}
	if len(loadbalancers[0].Frontends) == 0 {
		glog.Info("No service bind to this nginx now")
	}

	nginxConfig, ngxPolicies := nc.generateNginxConfig(loadbalancers[0])
	// glog.Infof("nginxConfig is %v", nginxConfig)
	// glog.Infof("policy is %v", ngxPolicies)

	policyBytes, err := json.Marshal(ngxPolicies)
	if err != nil {
		glog.Error()
		return err
	}
	configWriter, err := os.Create(nc.NewConfigPath)
	if err != nil {
		glog.Errorf("Failed to create new config file %s", err.Error())
		return err
	}
	defer configWriter.Close()
	policyWriter, err := os.Create(nc.NewPolicyPath)
	if err != nil {
		glog.Errorf("Failed to create new policy file %s", err.Error())
		return err
	}
	defer policyWriter.Close()

	t, err := template.New("nginx.tmpl").ParseFiles(nc.TemplatePath)
	if err != nil {
		glog.Errorf("Failed to parse template %s", err.Error())
		return err
	}
	if _, err := policyWriter.Write(policyBytes); err != nil {
		glog.Errorf("Write policy file failed %s", err.Error())
		return err
	}
	policyWriter.Sync()

	err = t.Execute(configWriter, nginxConfig)
	if err != nil {
		glog.Error(err)
		return err
	}
	if err := configWriter.Sync(); err != nil {
		glog.Error(err)
		return err
	}
	return nil
}

func (nc *NginxController) ReloadLoadBalancer() error {
	if nc.BinaryPath == "" {
		// set it to empty for local test
		glog.Errorf("Nginx bin path is empty!!!")
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

	pids, err := CheckProcessAlive(nc.BinaryPath)
	if err != nil && err.Error() != "exit status 1" {
		glog.Errorf("failed to check nginx aliveness: %s", err.Error())
		return err
	}
	pids = strings.Trim(pids, "\n ")
	configChanged := !sameFiles(nc.NewConfigPath, nc.OldConfigPath)

	// No change, Nginx running, skip
	if !configChanged && len(pids) > 0 && getLastReloadStatus(StatusFileParentPath) == SUCCESS {
		glog.Info("Config not changed and last reload success")
		return nil
	}

	// Update config and policy files
	if configChanged {
		diffOutput, _ := exec.Command("diff", "-u", nc.OldConfigPath, nc.NewConfigPath).CombinedOutput()
		glog.Infof("NGINX configuration diff\n")
		glog.Infof("%v\n", string(diffOutput))

		glog.Info("Start to change config.")
		err = os.Rename(nc.NewConfigPath, nc.OldConfigPath)
		if err != nil {
			glog.Errorf("failed to replace config: %s", err.Error())
			return err
		}
	}

	// Manipulate nginx
	if len(pids) == 0 {
		err = nc.start()
	} else if configChanged || getLastReloadStatus(StatusFileParentPath) == FAILED {
		err = nc.reload()
	}

	return err
}

func (nc *NginxController) start() error {
	glog.Info("Run command nginx start")
	output, err := exec.Command(nc.BinaryPath, "-c", nc.OldConfigPath).CombinedOutput()
	if err != nil {
		glog.Errorf("start nginx failed: %s %v", output, err)
	}
	return err
}

func (nc *NginxController) reload() error {
	glog.Info("Run command nginx -s reload")
	output, err := exec.Command(nc.BinaryPath, "-s", "reload", "-c", nc.OldConfigPath).CombinedOutput()
	if err != nil {
		glog.Errorf("start nginx failed: %s %v", output, err)
	}
	return err
}

func (nc *NginxController) GC() error {
	if config.Get("ENABLE_GC") != "true" {
		return nil
	}
	glog.Info("begin gc rule")
	return GCRule(nc.Driver)
}
