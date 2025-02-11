package controller

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"alauda.io/alb2/controller/cli"
	m "alauda.io/alb2/controller/modules"
	albv1 "alauda.io/alb2/pkg/apis/alauda/v1"
	"github.com/go-logr/logr"

	"alauda.io/alb2/config"
	"alauda.io/alb2/controller/state"
	. "alauda.io/alb2/controller/types"
	"alauda.io/alb2/driver"
	gateway "alauda.io/alb2/gateway/nginx"
	. "alauda.io/alb2/pkg/controller/ngxconf"
	. "alauda.io/alb2/pkg/controller/ngxconf/types"
	pm "alauda.io/alb2/pkg/utils/metrics"
	"k8s.io/klog/v2"
)

type NginxController struct {
	TemplatePath  string
	NewConfigPath string // in fact, the updated nginx.conf
	OldConfigPath string // in fact, the current nginx.conf
	NewPolicyPath string
	Driver        *driver.KubernetesDriver
	Ctx           context.Context
	albcfg        *config.Config
	log           logr.Logger
	lc            *LeaderElection
	PortProber    *PortProbe
	albcli        cli.AlbCli    // load alb tree from k8s
	policycli     cli.PolicyCli // fetch policy needed cr from k8s into alb tree
	ngxcli        NgxCli        // fetch ngxconf need cr from k8s into alb tree
}

func NewNginxController(kd *driver.KubernetesDriver, ctx context.Context, cfg *config.Config, log logr.Logger, leader *LeaderElection) *NginxController {
	ngx := cfg.GetNginxCfg()
	n := &NginxController{
		TemplatePath:  ngx.NginxTemplatePath,
		NewConfigPath: ngx.NewConfigPath,
		OldConfigPath: ngx.OldConfigPath,
		NewPolicyPath: ngx.NewPolicyPath,
		Driver:        kd,
		Ctx:           ctx,
		albcfg:        cfg,
		log:           log,
		lc:            leader,
		albcli:        cli.NewAlbCli(kd, log),
		policycli:     cli.NewPolicyCli(kd, log, cli.PolicyCliOpt{MetricsPort: cfg.GetMetricsPort()}),
		ngxcli:        NewNgxCli(kd, log, NgxCliOpt{}),
	}
	return n
}

func (nc *NginxController) GenerateConf() error {
	nginxConfig, ngxPolicies, err := nc.GenerateNginxConfigAndPolicy()
	if err != nil {
		return err
	}
	return nc.WriteConfig(nginxConfig, ngxPolicies)
}

// +-------------------------+      +--------------------------+
// | load from alb/ft/rule   |      |  load from gateway/route |
// |      (cli/albcli)       |      |      (gateway/nginx)     |
// +--------+----------------+      +--------------------------+
//
//	   |                                            |
//	   |   +----------------------------------+     |
//	   +-> |    types/loadbalancer            |<----+
//	       +--------------+-------------------+
//	                      |   fill with backend ips (cli/policy)
//	       +--------------v-------------------+
//	       |    types/loadbalancer            |
//	       +--------------+-------------------+
//	                      |   fill with cm which be referenced (cli/nginx)
//	       +--------------v-------------------+
//	       |    types/loadbalancer            |
//	       +--------------|-|-----------------+
//	            +---------+ +------------+
//	            |                        |
//	+-----------v---------+    +---------v------------+
//	|       policy.new    |    |    nginx.conf        |
//	+---------------------+    +----------------------+
func (nc *NginxController) GenerateNginxConfigAndPolicy() (nginxTemplateConfig NginxTemplateConfig, nginxPolicy NgxPolicy, err error) {
	alb, err := nc.GetLBConfig()
	l := nc.log
	if err != nil {
		return NginxTemplateConfig{}, NgxPolicy{}, err
	}
	if err = nc.policycli.FillUpBackends(alb); err != nil {
		return NginxTemplateConfig{}, NgxPolicy{}, err
	}

	if len(alb.Frontends) == 0 {
		l.Info("No service bind to this nginx now ", "key", nc.albcfg.GeKey())
	}
	nc.albcli.CollectAndFetchRefs(alb)

	nginxPolicy = nc.policycli.GenerateAlbPolicy(alb)
	phase := state.GetState().GetPhase()
	if phase != m.PhaseTerminating {
		phase = m.PhaseRunning
	}

	// TODO move to other goroutine
	if nc.albcfg.IsEnableVIP() && nc.lc != nil && nc.lc.AmILeader() {
		nc.log.Info("enable vip and I am the leader")
		if err := nc.SyncLbSvcPort(alb.Frontends); err != nil {
			nc.log.Error(err, "sync lb svc fail")
		}
	}

	cfg, err := nc.ngxcli.GenerateNginxTemplateConfig(alb, string(phase), nc.albcfg)
	if err != nil {
		return NginxTemplateConfig{}, NgxPolicy{}, fmt.Errorf("generate nginx.conf fail %v", err)
	}
	return *cfg, nginxPolicy, nil
}

func (nc *NginxController) GetLBConfig() (*LoadBalancer, error) {
	s := time.Now()
	defer func() {
		pm.Write("gen-lb-config", float64(time.Since(s).Milliseconds()))
	}()

	log := nc.log
	cfg := nc.albcfg
	ns := cfg.GetNs()
	name := cfg.GetAlbName()

	albEnable := cfg.IsEnableAlb()
	gcfg := cfg.GetGatewayCfg()
	gatewayEnable := gcfg.Enable

	log.Info("gen lb config", "ns", ns, "name", name, "alb", albEnable, "gateway", gatewayEnable, "networkmode", cfg.GetNetworkMode())
	if !albEnable && !gatewayEnable {
		return nil, fmt.Errorf("must enable at least one [gateway,alb]")
	}
	var lbFromAlb *LoadBalancer
	if albEnable {
		lb, err := nc.albcli.GetLBConfig(ns, name)
		if err != nil {
			return nil, err
		}
		lbFromAlb = lb
		// TODO: we should do it in a separate runner...
		nc.patrol(lb)
	}
	var lbFromGateway *LoadBalancer
	var err error
	if gatewayEnable {
		lbFromGateway, err = gateway.GetLBConfig(context.Background(), nc.Driver, cfg)
		if err != nil {
			log.Error(err, "get lb from gateway fail", "alb", name)
			return nil, err
		}
		log.Info("lb config from gateway ", "lbconfig", lbFromGateway)
	}

	if lbFromAlb == nil && lbFromGateway == nil {
		return nil, fmt.Errorf("alb and gateway both nil")
	}
	lb, err := MergeLBConfig(lbFromAlb, lbFromGateway)
	if err != nil {
		log.Error(err, "merge config fail ")
		return nil, err
	}
	log.V(3).Info("gen lb config ok", "lb-from-alb", lbFromAlb, "lb-from-gateway", lbFromGateway, "lb", lb)

	return lb, err
}

// alb or gateway could be nil
func MergeLBConfig(alb *LoadBalancer, gateway *LoadBalancer) (*LoadBalancer, error) {
	if alb == nil && gateway == nil {
		return nil, fmt.Errorf("alb and gateway are both nil")
	}
	if alb == nil && gateway != nil {
		return gateway, nil
	}
	if alb != nil && gateway == nil {
		return alb, nil
	}

	ftInAlb := make(map[string]*Frontend)
	for _, ft := range alb.Frontends {
		key := fmt.Sprintf("%v/%v", ft.Protocol, ft.Port)
		ftInAlb[key] = ft
	}
	for _, ft := range gateway.Frontends {
		key := fmt.Sprintf("%v/%v", ft.Protocol, ft.Port)
		albFt, find := ftInAlb[key]
		if find {
			http := ft.Protocol == albv1.FtProtocolHTTP || ft.Protocol == albv1.FtProtocolHTTPS
			// 其他协议都必须独享一个端口
			if !http {
				klog.Warningf("merge-gateway: find conflict port %v between gateway %v and alb %v ignore this gateway ft", ft.Port, ft.FtName, albFt.FtName)
				continue
			}
			ft.Rules = append(ft.Rules, albFt.Rules...)
		}
		alb.Frontends = append(alb.Frontends, ft)
	}
	return alb, nil
}

func (nc *NginxController) WriteConfig(nginxTemplateConfig NginxTemplateConfig, ngxPolicies NgxPolicy) error {
	s := time.Now()
	defer func() {
		pm.Write("update-file", float64(time.Since(s).Milliseconds()))
	}()
	ngxconf, err := RenderNgxFromFile(nginxTemplateConfig, nc.TemplatePath)
	if err != nil {
		return err
	}
	if err := os.WriteFile(nc.NewConfigPath, []byte(ngxconf), 0o644); err != nil {
		return err
	}

	if err := nc.UpdatePolicyFile(ngxPolicies); err != nil {
		klog.Error(err)
		return err
	}
	return nil
}

func (nc *NginxController) ReloadLoadBalancer() error {
	StatusFileParentPath := nc.albcfg.GetStatusFile()
	var err error
	defer func() {
		if err != nil {
			err := setLastReloadStatus(FAILED, StatusFileParentPath)
			if err != nil {
				klog.Errorf("failed to set last reload status to failed: %v", err)
			}
		} else {
			err := setLastReloadStatus(SUCCESS, StatusFileParentPath)
			if err != nil {
				klog.Errorf("failed to set last reload status to success: %v", err)
			}
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
		diffOutput, _ := exec.Command("diff", "-u", nc.OldConfigPath, nc.NewConfigPath).CombinedOutput() //nolint:errcheck
		klog.Infof("NGINX configuration diff\n")
		klog.Infof("%v\n", string(diffOutput))

		klog.Info("Start to change config.")
		err = os.Rename(nc.NewConfigPath, nc.OldConfigPath)
		if err != nil {
			klog.Errorf("failed to replace config: %s", err.Error())
			return err
		}
	}

	if nc.albcfg.GetFlags().E2eTestControllerOnly {
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

func GetProcessId() (string, error) {
	process := "/etc/alb2/nginx/nginx.pid"
	out, err := os.ReadFile(process)
	if err != nil {
		klog.Errorf("nginx process is not started: %s", err.Error())
		return "", err
	}
	return string(out), err
}
