package controller

import (
	"alb2/config"
	"alb2/driver"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"sort"
	"strings"
	"text/template"
	"time"

	"fmt"
	"io/ioutil"

	"github.com/golang/glog"
	"github.com/parnurzeal/gorequest"
)

var (
	NginxRequest = gorequest.New().Timeout(5 * time.Second)
)

const DEFAULT_RULE = "(STARTS_WITH URL /)"

type NginxController struct {
	BackendType   string
	TemplatePath  string
	NewConfigPath string
	OldConfigPath string
	NewPolicyPath string
	OldPolicyPath string
	BinaryPath    string
	Driver        *driver.KubernetesDriver
}

type TrafficRule struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type Policy struct {
	Rule          string `json:"rule"`
	Upstream      string `json:"upstream"`
	URL           string `json:"url"`
	RewriteTarget string `json:"rewrite_target"`
	Priority      int    `json:"priority"`
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
	// TODO: 需要新的ngxpolicy，包含upstream信息的
	config := generateConfig(loadbalancer, nc.Driver)
	ngxPolicy := NgxPolicy{
		CertificateMap: config.CertificateMap,
		PortMap:        make(map[int]Policies),
		BackendGroup:   config.BackendGroup,
	}
	for port, frontend := range config.Frontends {
		if frontend.Protocol == ProtocolTCP {
			continue
		}
		glog.V(3).Infof("Frontend is %+v", frontend)
		if _, ok := ngxPolicy.PortMap[port]; !ok {
			ngxPolicy.PortMap[port] = Policies{}
		}
		glog.V(4).Infof("Rules are %+v", frontend.Rules)
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
			}

			if rule.DSL == "" {
				continue
			}

			glog.V(3).Infof("Rule is %v", rule)
			policy := Policy{}
			// it's using id as the name of certificate file now..
			policy.Rule = rule.DSL
			if rule.Priority != 0 {
				policy.Priority = int(rule.Priority)
			} else {
				policy.Priority = len(rule.DSL)
			}
			policy.Upstream = rule.BackendGroup.Name
			// for rewrite
			policy.URL = rule.URL
			policy.RewriteTarget = rule.RewriteTarget
			ngxPolicy.PortMap[port] = append(ngxPolicy.PortMap[port], &policy)
		}

		// set default rule if exists
		if frontend.BackendGroup != nil {
			glog.V(3).Infof("Default rule is %v", frontend.BackendGroup)
			policy := Policy{}
			policy.Rule = DEFAULT_RULE
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

	// As updatePolicyAndCerts is a cheap operation,
	// always update policy to avoid some failure.
	defer nc.updatePolicy()

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

func (nc *NginxController) updatePolicy() error {
	policyUrl := "http://127.0.0.1:1936/policies"
	resp, body, errs := NginxRequest.Get(policyUrl).End()
	if len(errs) > 0 {
		glog.Errorf("Get nginx policy failed %v", errs)
		return errs[0]
	}
	if resp.StatusCode != 200 {
		glog.Errorf("Get nginx policy failed %s", body)
		return errors.New(body)
	}
	oldPolicy := []byte(body)
	newPolicy, err := ioutil.ReadFile(nc.NewPolicyPath)
	if err != nil {
		glog.Errorf("failed to open policy file %v", err)
	}
	if len(newPolicy) > 0 && !jsonEqual(oldPolicy, newPolicy) {
		glog.Infof("Policy changed, old policy is: \n %s", oldPolicy)
		glog.Infof("Policy changed, new policy is: \n %s", newPolicy)
		resp, body, errs = NginxRequest.Put(policyUrl).Send(string(newPolicy)).End()
		if len(errs) > 0 {
			glog.Errorf("Update nginx policy failed %v", errs)
			return errs[0]
		}
		if resp.StatusCode != 200 {
			glog.Errorf("update nginx policy failed %s", body)
			return errors.New(body)
		}
	} else {
		glog.Info("Nginx policy not change.")
	}
	return nil
}

func (nc *NginxController) GC() error {
	if config.Get("ENABLE_GC") != "true" {
		return nil
	}
	glog.Info("begin gc rule")
	return GCRule(nc.Driver)
}
