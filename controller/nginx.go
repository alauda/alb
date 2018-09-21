package controller

import (
	"alauda_lb/config"
	"alauda_lb/driver"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
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
	Driver        driver.Driver
}

type TrafficRule struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type Policy struct {
	Rule     string `json:"rule"`
	Upstream string `json:"upstream"`
}

func (nc *NginxController) GetLoadBalancerType() string {
	return "Nginx"
}

func generateNginxConfig(loadbalancer *LoadBalancer) (Config, map[int][]*Policy) {
	config := generateConfig(loadbalancer)
	httpPortPolicies := map[int][]*Policy{}
	for port, frontend := range config.Frontends {
		if frontend.Protocol == ProtocolTCP {
			continue
		}
		glog.Infof("Frontend is %v", frontend)
		if _, ok := httpPortPolicies[port]; !ok {
			httpPortPolicies[port] = []*Policy{}
		}
		glog.Infof("Rules are %v", frontend.Rules)
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

			glog.Infof("Rule is %v", rule)
			policy := Policy{}
			// it's using id as the name of certificate file now..
			policy.Rule = rule.DSL
			policy.Upstream = rule.BackendGroup.Name
			httpPortPolicies[port] = append(httpPortPolicies[port], &policy)
		}

		// set default rule if exists
		if frontend.BackendGroup != nil {
			glog.Infof("Default rule is %v", frontend.BackendGroup)
			policy := Policy{}
			policy.Rule = DEFAULT_RULE
			policy.Upstream = frontend.BackendGroup.Name
			httpPortPolicies[port] = append(httpPortPolicies[port], &policy)
		}

	}
	return config, httpPortPolicies
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
	loadbalancers = filterLoadbalancers(loadbalancers, "nginx", config.Get("NAME"))
	if len(loadbalancers) == 0 {
		return errors.New("No loadbalancer config related to this nginx")
	}

	merge(loadbalancers, services)
	if err != nil {
		return err
	}

	if len(loadbalancers[0].Frontends) == 0 {
		glog.Info("No service bind to this nginx now")
	}

	nginxConfig, httpPortPolicies := generateNginxConfig(loadbalancers[0])
	glog.Infof("nginxConfig is %v", nginxConfig)
	glog.Infof("policy is %v", httpPortPolicies)

	policyBytes, err := json.Marshal(httpPortPolicies)
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
