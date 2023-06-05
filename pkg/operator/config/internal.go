package config

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/go-logr/logr"

	. "alauda.io/alb2/pkg/apis/alauda/v2beta1"
	"alauda.io/alb2/pkg/operator/toolkit"
	"alauda.io/alb2/utils"
	coreV1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

type OverwriteCfg ExternalOverwrite

// tree like config use in operator.
type ALB2Config struct {
	Name       string
	Ns         string
	Deploy     DeployConfig
	Vip        VipConfig
	Controller ControllerConfig
	Project    ProjectConfig
	Gateway    GatewayConfig
	Overwrite  OverwriteCfg
}

type GatewayConfig struct {
	Enable        bool           `json:"enable"`
	Mode          string         `json:"mode"`
	GatewayModeCg GatewayModeCfg `json:"gatewayMode"`
}

type GatewayModeCfg struct {
	Name string `json:"name"`
}

type ProjectConfig struct {
	EnablePortProject bool     `json:"enablePortProject,omitempty"`
	PortProjects      string   `json:"portProjects,omitempty"` // json string of []struct {Port:stringProjects:[]string}
	Projects          []string `json:"projects,omitempty"`
}

type DeployConfig struct {
	Replicas        int
	AntiAffinityKey string
	ALbResource     coreV1.ResourceRequirements
	NginxResource   coreV1.ResourceRequirements
	NodeSelector    map[string]string
}

type ControllerConfig struct {
	NetworkMode        string
	MetricsPort        int
	BindNic            string
	HttpPort           int
	HttpsPort          int
	SSLCert            string
	DefaultSSLStrategy string
	MaxTermSeconds     int
	CpuPreset          int
	WorkerLimit        int
	GoMonitorPort      int
	ResyncPeriod       int
	Flags              ControllerFlags
	BackLog            int
}

type ControllerFlags struct {
	EnableAlb           bool
	EnableGC            bool
	EnableGCAppRule     bool
	EnablePrometheus    bool
	EnablePortprobe     bool
	EnablePortProject   bool
	EnableIPV6          bool
	EnableHTTP2         bool
	EnableIngress       bool
	EnableCrossClusters bool
	EnableGzip          bool
	EnableGoMonitor     bool
	EnableProfile       bool
	PolicyZip           bool
}

type IngressConfig struct {
	Enable bool
}

func NewALB2Config(albCr *ALB2, log logr.Logger) (*ALB2Config, error) {
	external, err := NewExternalAlbConfigWithDefault(albCr)
	if err != nil {
		return nil, err
	}
	cfg := &ALB2Config{
		Name: albCr.Name,
		Ns:   albCr.Namespace,
	}
	err = cfg.Merge(external)
	if err != nil {
		return nil, err
	}
	log.V(3).Info("config", "merged", utils.PrettyJson(external))
	log.Info("config", "internal", utils.PrettyJson(cfg))
	return cfg, nil
}

func (a *ALB2Config) Merge(ec ExternalAlbConfig) error {
	a.Name = *ec.LoadbalancerName

	a.Deploy = DeployConfig{}
	a.Project = ProjectConfig{}
	a.Gateway = GatewayConfig{}
	a.Vip = VipConfig{}
	if ec.Vip != nil {
		a.Vip = *ec.Vip
	}
	a.Controller.Merge(ec)
	err := a.Deploy.Merge(ec)
	if err != nil {
		return err
	}
	a.Project.Merge(ec)
	a.Gateway.Merge(ec, a.Ns)
	if ec.Overwrite != nil {
		a.Overwrite = OverwriteCfg(*ec.Overwrite)
	}
	return nil
}

func (c *ControllerConfig) Merge(ec ExternalAlbConfig) {
	c.BackLog = *ec.Backlog
	c.NetworkMode = *ec.NetworkMode
	c.MetricsPort = *ec.MetricsPort
	c.BindNic = *ec.BindNIC
	c.HttpPort = *ec.IngressHTTPPort
	c.HttpsPort = *ec.IngressHTTPSPort
	c.SSLCert = *ec.DefaultSSLCert
	c.DefaultSSLStrategy = *ec.DefaultSSLStrategy
	c.MaxTermSeconds = *ec.MaxTermSeconds
	c.CpuPreset = toolkit.CpuPresetToCore(ec.Resources.Limits.CPU)
	c.WorkerLimit = *ec.WorkerLimit
	c.ResyncPeriod = *ec.ResyncPeriod
	c.GoMonitorPort = *ec.GoMonitorPort
	c.Flags = ControllerFlags{}
	c.Flags.Merge(ec)
}

func (d *DeployConfig) Merge(ec ExternalAlbConfig) error {
	ngxLimitCpu, err := resource.ParseQuantity(ec.Resources.Limits.CPU)
	if err != nil {
		return err
	}
	ngxLimMem, err := resource.ParseQuantity(ec.Resources.Limits.Memory)
	if err != nil {
		return err
	}
	ngxReqCpu, err := resource.ParseQuantity(ec.Resources.Requests.CPU)
	if err != nil {
		return err
	}
	ngxReqMem, err := resource.ParseQuantity(ec.Resources.Requests.Memory)
	if err != nil {
		return err
	}

	albLimitCpu, err := resource.ParseQuantity(ec.Resources.Alb.Limits.CPU)
	if err != nil {
		return err
	}
	albLimitMem, err := resource.ParseQuantity(ec.Resources.Alb.Limits.Memory)
	if err != nil {
		return err
	}
	albReqCpu, err := resource.ParseQuantity(ec.Resources.Alb.Requests.CPU)
	if err != nil {
		return err
	}
	albReqMem, err := resource.ParseQuantity(ec.Resources.Alb.Requests.Memory)
	if err != nil {
		return err
	}
	d.NginxResource = coreV1.ResourceRequirements{
		Limits: coreV1.ResourceList{
			coreV1.ResourceCPU:    ngxLimitCpu,
			coreV1.ResourceMemory: ngxLimMem,
		},
		Requests: coreV1.ResourceList{
			coreV1.ResourceCPU:    ngxReqCpu,
			coreV1.ResourceMemory: ngxReqMem,
		},
	}
	d.ALbResource = coreV1.ResourceRequirements{
		Limits: coreV1.ResourceList{
			coreV1.ResourceCPU:    albLimitCpu,
			coreV1.ResourceMemory: albLimitMem,
		},
		Requests: coreV1.ResourceList{
			coreV1.ResourceCPU:    albReqCpu,
			coreV1.ResourceMemory: albReqMem,
		},
	}
	d.Replicas = *ec.Replicas
	d.AntiAffinityKey = *ec.AntiAffinityKey
	d.NodeSelector = ec.NodeSelector
	return nil
}

func (f *ControllerFlags) Merge(ec ExternalAlbConfig) {
	f.EnableAlb = toBool(*ec.EnableAlb)
	f.EnableGC = toBool(*ec.EnableGC)
	f.EnableGCAppRule = toBool(*ec.EnableGCAppRule)
	f.EnablePrometheus = toBool(*ec.EnablePrometheus)
	f.EnablePortprobe = toBool(*ec.EnablePortprobe)
	f.EnablePortProject = toBool(*ec.EnablePortProject)
	f.EnableIPV6 = toBool(*ec.EnableIPV6)
	f.EnableHTTP2 = toBool(*ec.EnableHTTP2)
	f.EnableCrossClusters = toBool(*ec.EnableCrossClusters)
	f.EnableGzip = toBool(*ec.EnableGzip)
	f.EnableGoMonitor = toBool(*ec.EnableGoMonitor)
	f.EnableProfile = toBool(*ec.EnableProfile)
	f.EnableIngress = toBool(*ec.EnableIngress)
	f.PolicyZip = toBool(*ec.PolicyZip)
}

func (d *ProjectConfig) Merge(ec ExternalAlbConfig) {
	d.EnablePortProject = *ec.EnablePortProject
	d.Projects = ec.Projects
	d.PortProjects = *ec.PortProjects
}

func (g *GatewayConfig) Merge(ec ExternalAlbConfig, ns string) {
	g.Enable = ec.Gateway.Enable
	if !g.Enable {
		return
	}
	// 兼容性配置
	// 当gateway模式开启，但是没有指定gatewaymode的时候，默认用gatewayclassmode
	// gatewayclass模式下name默认是alb的name
	// gateway模式下要有指定的name
	if ec.Gateway.Mode == nil || *ec.Gateway.Mode == "" {
		g.Mode = "gatewayclass"
		g.GatewayModeCg = GatewayModeCfg{
			Name: *ec.LoadbalancerName,
		}
	} else {
		g.Mode = *ec.Gateway.Mode
		if ec.Gateway.GatewayModeCfg != nil {
			g.GatewayModeCg = GatewayModeCfg{
				Name: ec.Gateway.GatewayModeCfg.Name,
			}
		} else {
			if g.Mode == "gatewayclass" {
				g.GatewayModeCg = GatewayModeCfg{
					Name: *ec.LoadbalancerName,
				}
			} else {
				g.GatewayModeCg = GatewayModeCfg{
					Name: fmt.Sprintf("%s/%s", ns, *ec.LoadbalancerName),
				}
			}
		}
	}
}

func (a *ALB2Config) Show() string {
	return fmt.Sprintf("%+v", a)
}

func toBool(x interface{}) bool {
	switch x.(type) {
	case string:
		bs := strings.ToLower(x.(string))
		return bs == "true"
	case bool:
		return x.(bool)
	default:
		panic("? what is you type")
	}
}

func toEnv(x interface{}) string {
	switch x.(type) {
	case string:
		return x.(string)
	case bool:
		return fmt.Sprintf("%v", x)
	case int:
		return fmt.Sprintf("%d", x)
	default:
		panic("? what is you type")
	}
}

// 通用环境变量
func (a *ALB2Config) aLBContainerCommonEnvs(env OperatorCfg) []coreV1.EnvVar {
	var envs []coreV1.EnvVar
	envs = append(envs,
		coreV1.EnvVar{Name: "MAX_TERM_SECONDS", Value: toEnv(a.Controller.MaxTermSeconds)},
		coreV1.EnvVar{Name: "ENABLE_GC", Value: toEnv(a.Controller.Flags.EnableGC)},
		coreV1.EnvVar{Name: "ENABLE_GC_APP_RULE", Value: toEnv(a.Controller.Flags.EnableGCAppRule)},
		coreV1.EnvVar{Name: "ENABLE_PROMETHEUS", Value: toEnv(a.Controller.Flags.EnablePrometheus)},
		coreV1.EnvVar{Name: "ENABLE_PORTPROBE", Value: toEnv(a.Controller.Flags.EnablePortprobe)},
		coreV1.EnvVar{Name: "ENABLE_IPV6", Value: toEnv(a.Controller.Flags.EnableIPV6)},
		coreV1.EnvVar{Name: "ENABLE_HTTP2", Value: toEnv(a.Controller.Flags.EnableHTTP2)},
		coreV1.EnvVar{Name: "ENABLE_GZIP", Value: toEnv(a.Controller.Flags.EnableGzip)},
		coreV1.EnvVar{Name: "ENABLE_GO_MONITOR", Value: toEnv(a.Controller.Flags.EnableGoMonitor)},
		coreV1.EnvVar{Name: "ENABLE_PROFILE", Value: toEnv(a.Controller.Flags.EnableProfile)},
		coreV1.EnvVar{Name: "GO_MONITOR_PORT", Value: toEnv(a.Controller.GoMonitorPort)},
		coreV1.EnvVar{Name: "BACKLOG", Value: toEnv(a.Controller.BackLog)},
		coreV1.EnvVar{Name: "POLICY_ZIP", Value: toEnv(a.Controller.Flags.PolicyZip)},
		coreV1.EnvVar{Name: "SERVE_CROSSCLUSTERS", Value: toEnv(a.Controller.Flags.EnableCrossClusters)},
		coreV1.EnvVar{Name: "SERVE_INGRESS", Value: toEnv(a.Controller.Flags.EnableIngress)},
		coreV1.EnvVar{Name: "METRICS_PORT", Value: toEnv(a.Controller.MetricsPort)},
		coreV1.EnvVar{Name: "RESYNC_PERIOD", Value: toEnv(a.Controller.ResyncPeriod)},
	)
	return envs
}

// TODO move to workload?
// TODO move to config?
func (a *ALB2Config) GetALBContainerEnvs(env OperatorCfg) []coreV1.EnvVar {
	var envs []coreV1.EnvVar

	envs = append(envs, a.aLBContainerCommonEnvs(env)...)

	envs = append(envs,
		coreV1.EnvVar{Name: "DEFAULT-SSL-STRATEGY", Value: a.Controller.DefaultSSLStrategy},
		coreV1.EnvVar{Name: "DEFAULT-SSL-CERTIFICATE", Value: a.Controller.SSLCert},
	)

	envs = append(envs,
		coreV1.EnvVar{Name: "MODE", Value: "controller"},
		coreV1.EnvVar{Name: "NAMESPACE", Value: a.Ns}, // 在测试中要读ns的环境变量 这里就直接设置string而不是valuesfrom
		coreV1.EnvVar{Name: "NAME", Value: a.Name},
		coreV1.EnvVar{Name: "MY_POD_NAME", ValueFrom: &coreV1.EnvVarSource{FieldRef: &coreV1.ObjectFieldSelector{FieldPath: "metadata.name", APIVersion: "v1"}}},
		coreV1.EnvVar{Name: "WORKER_LIMIT", Value: toEnv(a.Controller.WorkerLimit)}, // nginx 真正使用的worker process 是min(cpupreset,worklimit)
		coreV1.EnvVar{Name: "CPU_PRESET", Value: toEnv(a.Controller.CpuPreset)},
		coreV1.EnvVar{Name: "NETWORK_MODE", Value: a.Controller.NetworkMode},
		coreV1.EnvVar{Name: "DOMAIN", Value: env.BaseDomain},
		coreV1.EnvVar{Name: "INGRESS_HTTP_PORT", Value: toEnv(a.Controller.HttpPort)},
		coreV1.EnvVar{Name: "INGRESS_HTTPS_PORT", Value: toEnv(a.Controller.HttpsPort)},
		coreV1.EnvVar{Name: "ALB_ENABLE", Value: toEnv(a.Controller.Flags.EnableAlb)},
		coreV1.EnvVar{Name: "GATEWAY_ENABLE", Value: strconv.FormatBool(a.Gateway.Enable)},
	)
	if a.Gateway.Enable {
		envs = append(envs,
			coreV1.EnvVar{Name: "GATEWAY_MODE", Value: a.Gateway.Mode},
			coreV1.EnvVar{Name: "GATEWAY_NAME", Value: a.Gateway.GatewayModeCg.Name},
		)
	}
	envs = append(envs, coreV1.EnvVar{Name: "ENABLE_VIP", Value: toEnv(a.Vip.EnableLbSvc)})
	return envs
}

func (a *ALB2Config) GetNginxContainerEnvs() []coreV1.EnvVar {
	var envs []coreV1.EnvVar
	envs = append(envs,
		coreV1.EnvVar{Name: "SYNC_POLICY_INTERVAL", Value: "1"},
		coreV1.EnvVar{Name: "CLEAN_METRICS_INTERVAL", Value: toEnv(2592000)},
		coreV1.EnvVar{Name: "NEW_POLICY_PATH", Value: "/etc/alb2/nginx/policy.new"},
		coreV1.EnvVar{Name: "POLICY_ZIP", Value: toEnv(a.Controller.Flags.PolicyZip)},
		coreV1.EnvVar{Name: "DEFAULT-SSL-STRATEGY", Value: a.Controller.DefaultSSLStrategy},
		coreV1.EnvVar{Name: "INGRESS_HTTPS_PORT", Value: toEnv(a.Controller.HttpsPort)},
		coreV1.EnvVar{Name: "MAX_TERM_SECONDS", Value: toEnv(a.Controller.MaxTermSeconds)},
	)
	return envs
}
