package config

import (
	"fmt"
	"strconv"
	"strings"

	. "alauda.io/alb2/pkg/apis/alauda/v2beta1"
	coreV1 "k8s.io/api/core/v1"
)

// TODO 通过生成器或者反射的方式减少代码

// config used in alb container

type ALBRunConfig struct {
	Name       string
	Ns         string
	Domain     string
	Controller ControllerConfig
	Gateway    GatewayConfig
}

type GatewayConfig struct {
	Enable     bool                     `json:"enable"`
	Mode       GatewayMode              `json:"mode"`
	StandAlone *GatewayStandAloneConfig `json:"standAlone"`
	Shared     *GatewaySharedConfig     `json:"shared"`
}

type GatewayStandAloneConfig struct {
	GatewayName string `json:"name"`
	GatewayNS   string `json:"ns"`
}

type GatewaySharedConfig struct {
	GatewayClassName string `json:"name"`
}

type ControllerConfig struct {
	NetworkMode        string
	MetricsPort        int
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
	EnablePortProbe     bool
	EnablePortProject   bool
	EnableIPV6          bool
	EnableHTTP2         bool
	EnableIngress       bool
	EnableCrossClusters bool
	EnableGzip          bool
	EnableGoMonitor     bool
	EnableProfile       bool
	PolicyZip           bool
	EnableLbSvc         bool

	DebugRuleSync bool
}

type IngressConfig struct {
	Enable bool
}

const (
	MAX_TERM_SECONDS        string = "MAX_TERM_SECONDS"
	ENABLE_GC               string = "ENABLE_GC"
	ENABLE_GC_APP_RULE      string = "ENABLE_GC_APP_RULE"
	ENABLE_PROMETHEUS       string = "ENABLE_PROMETHEUS"
	ENABLE_PORTPROBE        string = "ENABLE_PORTPROBE"
	ENABLE_IPV6             string = "ENABLE_IPV6"
	ENABLE_HTTP2            string = "ENABLE_HTTP2"
	ENABLE_GZIP             string = "ENABLE_GZIP"
	ENABLE_GO_MONITOR       string = "ENABLE_GO_MONITOR"
	ENABLE_PROFILE          string = "ENABLE_PROFILE"
	GO_MONITOR_PORT         string = "GO_MONITOR_PORT"
	BACKLOG                 string = "BACKLOG"
	POLICY_ZIP              string = "POLICY_ZIP"
	SERVE_CROSSCLUSTERS     string = "SERVE_CROSSCLUSTERS"
	SERVE_INGRESS           string = "SERVE_INGRESS"
	METRICS_PORT            string = "METRICS_PORT"
	RESYNC_PERIOD           string = "RESYNC_PERIOD"
	DEFAULT_SSL_STRATEGY    string = "DEFAULT_SSL_STRATEGY"
	DEFAULT_SSL_CERTIFICATE string = "DEFAULT-SSL-CERTIFICATE"
	DOMAIN                  string = "DOMAIN"
	NAMESPACE               string = "NAMESPACE"
	NAME                    string = "NAME"
	MY_POD_NAME             string = "MY_POD_NAME"
	WORKER_LIMIT            string = "WORKER_LIMIT"
	CPU_PRESET              string = "CPU_PRESET"
	NETWORK_MODE            string = "NETWORK_MODE"
	INGRESS_HTTP_PORT       string = "INGRESS_HTTP_PORT"
	INGRESS_HTTPS_PORT      string = "INGRESS_HTTPS_PORT"
	ALB_ENABLE              string = "ALB_ENABLE"
	GATEWAY_ENABLE          string = "GATEWAY_ENABLE"
	GATEWAY_MODE            string = "GATEWAY_MODE"
	GATEWAY_CLASSNAME       string = "GATEWAY_CLASSNAME"
	GATEWAY_NAME            string = "GATEWAY_NAME"
	GATEWAY_NS              string = "GATEWAY_NS"
	ENABLE_VIP              string = "ENABLE_VIP"
	SYNC_POLICY_INTERVAL    string = "SYNC_POLICY_INTERVAL"
	CLEAN_METRICS_INTERVAL  string = "CLEAN_METRICS_INTERVAL"
	// flags used in e2e test only
	RELOAD_NGINX                    string = "RELOAD_NGINX"
	DISABLE_PERIOD_GEN_NGINX_CONFIG string = "DISABLE_PERIOD_GEN_NGINX_CONFIG"
	FULL_SYNC                       string = "FULL_SYNC"
	E2E_TEST_CONTROLLER_ONLY        string = "E2E_TEST_CONTROLLER_ONLY"

	// alb 自己知道的配置,不应该由oprator来设置
	DEBUG_RULESYNC string = "DEBUG_RULESYNC"
)

// alb 内部使用的一些配置
const (
	INTERVAL            string = "INTERVAL"
	RELOAD_TIMEOUT      string = "RELOAD_TIMEOUT"
	NEW_POLICY_PATH     string = "NEW_POLICY_PATH"
	NEW_CONFIG_PATH     string = "NEW_CONFIG_PATH"
	OLD_CONFIG_PATH     string = "OLD_CONFIG_PATH"
	TWEAK_DIR           string = "TWEAK_DIR"
	NGINX_TEMPLATE_PATH string = "NGINX_TEMPLATE_PATH"
	USE_KUBE_CONFIG     string = "USE_KUBE_CONFIG"
	KUBE_CONFIG         string = "KUBE_CONFIG"
	KUBE_SERVER         string = "KUBE_SERVER"
	KUBE_TOKEN          string = "KUBE_TOKEN"

	NGINX_TEMPLATE_PATH_VAL     string = "/alb/ctl/template/nginx/nginx.tmpl"
	NEW_POLICY_PATH_VAL         string = "/etc/alb2/nginx/policy.new"
	NEW_CONFIG_PATH_VAL         string = "/etc/alb2/nginx/nginx.conf.new"
	OLD_CONFIG_PATH_VAL         string = "/etc/alb2/nginx/nginx.conf"
	STATUS_FILE_PARENT_PATH_VAL string = "/etc/alb2/nginx/last_status"
	TWEAK_DIR_VAL               string = "/alb/tweak/"
	INTERVAL_VAL                int    = 5
	RELOAD_TIMEOUT_VAL          int    = 30
)

func (a *ALBRunConfig) GetALBContainerEnvs() []coreV1.EnvVar {
	var envs []coreV1.EnvVar

	envs = append(envs,
		coreV1.EnvVar{Name: NAME, Value: a.Name},
		coreV1.EnvVar{Name: NAMESPACE, Value: a.Ns}, // 在测试中要读ns的环境变量 这里就直接设置string而不是valuesfrom
		coreV1.EnvVar{Name: DOMAIN, Value: a.Domain},

		coreV1.EnvVar{Name: NETWORK_MODE, Value: a.Controller.NetworkMode},

		coreV1.EnvVar{Name: ENABLE_VIP, Value: BoolToEnv(a.Controller.Flags.EnableLbSvc)},

		coreV1.EnvVar{Name: DEFAULT_SSL_STRATEGY, Value: a.Controller.DefaultSSLStrategy},
		coreV1.EnvVar{Name: DEFAULT_SSL_CERTIFICATE, Value: a.Controller.SSLCert},
		coreV1.EnvVar{Name: MAX_TERM_SECONDS, Value: IntToEnv(a.Controller.MaxTermSeconds)},
		coreV1.EnvVar{Name: ENABLE_GC, Value: BoolToEnv(a.Controller.Flags.EnableGC)},
		coreV1.EnvVar{Name: ENABLE_GC_APP_RULE, Value: BoolToEnv(a.Controller.Flags.EnableGCAppRule)},
		coreV1.EnvVar{Name: ENABLE_PROMETHEUS, Value: BoolToEnv(a.Controller.Flags.EnablePrometheus)},
		coreV1.EnvVar{Name: ENABLE_PORTPROBE, Value: BoolToEnv(a.Controller.Flags.EnablePortProbe)},
		coreV1.EnvVar{Name: ENABLE_IPV6, Value: BoolToEnv(a.Controller.Flags.EnableIPV6)},
		coreV1.EnvVar{Name: ENABLE_HTTP2, Value: BoolToEnv(a.Controller.Flags.EnableHTTP2)},
		coreV1.EnvVar{Name: ENABLE_GZIP, Value: BoolToEnv(a.Controller.Flags.EnableGzip)},
		coreV1.EnvVar{Name: ENABLE_GO_MONITOR, Value: BoolToEnv(a.Controller.Flags.EnableGoMonitor)},
		coreV1.EnvVar{Name: ENABLE_PROFILE, Value: BoolToEnv(a.Controller.Flags.EnableProfile)},
		coreV1.EnvVar{Name: GO_MONITOR_PORT, Value: IntToEnv(a.Controller.GoMonitorPort)},
		coreV1.EnvVar{Name: BACKLOG, Value: IntToEnv(a.Controller.BackLog)},
		coreV1.EnvVar{Name: POLICY_ZIP, Value: BoolToEnv(a.Controller.Flags.PolicyZip)},
		coreV1.EnvVar{Name: SERVE_CROSSCLUSTERS, Value: BoolToEnv(a.Controller.Flags.EnableCrossClusters)},
		coreV1.EnvVar{Name: SERVE_INGRESS, Value: BoolToEnv(a.Controller.Flags.EnableIngress)},
		coreV1.EnvVar{Name: METRICS_PORT, Value: IntToEnv(a.Controller.MetricsPort)},
		coreV1.EnvVar{Name: RESYNC_PERIOD, Value: IntToEnv(a.Controller.ResyncPeriod)},
		coreV1.EnvVar{Name: WORKER_LIMIT, Value: IntToEnv(a.Controller.WorkerLimit)}, // nginx 真正使用的worker process 是min(cpupreset,worklimit)
		coreV1.EnvVar{Name: CPU_PRESET, Value: IntToEnv(a.Controller.CpuPreset)},
		coreV1.EnvVar{Name: INGRESS_HTTP_PORT, Value: IntToEnv(a.Controller.HttpPort)},
		coreV1.EnvVar{Name: INGRESS_HTTPS_PORT, Value: IntToEnv(a.Controller.HttpsPort)},
		coreV1.EnvVar{Name: ALB_ENABLE, Value: BoolToEnv(a.Controller.Flags.EnableAlb)},
		coreV1.EnvVar{Name: MY_POD_NAME, ValueFrom: &coreV1.EnvVarSource{FieldRef: &coreV1.ObjectFieldSelector{FieldPath: "metadata.name", APIVersion: "v1"}}},
	)

	envs = append(envs,
		coreV1.EnvVar{Name: GATEWAY_ENABLE, Value: BoolToEnv(a.Gateway.Enable)},
	)
	if a.Gateway.Enable {
		envs = append(envs,
			coreV1.EnvVar{Name: GATEWAY_MODE, Value: string(a.Gateway.Mode)},
		)
		if a.Gateway.Shared != nil {
			envs = append(envs,
				coreV1.EnvVar{Name: GATEWAY_CLASSNAME, Value: a.Gateway.Shared.GatewayClassName},
			)
		}
		if a.Gateway.StandAlone != nil {
			envs = append(envs,
				coreV1.EnvVar{Name: GATEWAY_NAME, Value: a.Gateway.StandAlone.GatewayName},
				coreV1.EnvVar{Name: GATEWAY_NS, Value: a.Gateway.StandAlone.GatewayNS},
			)
		}
	}
	return envs
}

func (a *ALBRunConfig) GetNginxContainerEnvs() []coreV1.EnvVar {
	var envs []coreV1.EnvVar
	envs = append(envs,
		coreV1.EnvVar{Name: OLD_CONFIG_PATH, Value: OLD_CONFIG_PATH_VAL},
		coreV1.EnvVar{Name: SYNC_POLICY_INTERVAL, Value: "1"},
		coreV1.EnvVar{Name: CLEAN_METRICS_INTERVAL, Value: IntToEnv(2592000)},
		coreV1.EnvVar{Name: DEFAULT_SSL_STRATEGY, Value: a.Controller.DefaultSSLStrategy},
		coreV1.EnvVar{Name: INGRESS_HTTPS_PORT, Value: IntToEnv(a.Controller.HttpsPort)},
		coreV1.EnvVar{Name: MAX_TERM_SECONDS, Value: IntToEnv(a.Controller.MaxTermSeconds)},
		coreV1.EnvVar{Name: POLICY_ZIP, Value: BoolToEnv(a.Controller.Flags.PolicyZip)},
		coreV1.EnvVar{Name: NEW_POLICY_PATH, Value: NEW_POLICY_PATH_VAL},
	)
	return envs
}

func AlbRunCfgFromEnv(env map[string]string) (ALBRunConfig, error) {
	c := ALBRunConfig{}
	c.Domain = env[DOMAIN]
	c.Ns = env[NAMESPACE]
	c.Name = env[NAME]
	controllerFromEnv(env, &c)
	gatewayFromEnv(env, &c)
	return c, nil
}

func controllerFromEnv(env map[string]string, c *ALBRunConfig) {
	c.Controller.BackLog = ToInt(env[BACKLOG])
	c.Controller.NetworkMode = env[NETWORK_MODE]
	c.Controller.MetricsPort = ToInt(env[METRICS_PORT])
	c.Controller.HttpPort = ToInt(env[INGRESS_HTTP_PORT])
	c.Controller.HttpsPort = ToInt(env[INGRESS_HTTPS_PORT])
	c.Controller.SSLCert = env[DEFAULT_SSL_CERTIFICATE]
	c.Controller.DefaultSSLStrategy = env[DEFAULT_SSL_STRATEGY]
	c.Controller.MaxTermSeconds = ToInt(env[MAX_TERM_SECONDS])
	c.Controller.CpuPreset = ToInt(env[CPU_PRESET])
	c.Controller.WorkerLimit = ToInt(env[WORKER_LIMIT])
	c.Controller.ResyncPeriod = ToInt(env[RESYNC_PERIOD])
	c.Controller.GoMonitorPort = ToInt(env[GO_MONITOR_PORT])
	flagsFromEnv(env, c)
}

func flagsFromEnv(env map[string]string, c *ALBRunConfig) {
	c.Controller.Flags.EnableAlb = ToBool(env[ALB_ENABLE])
	c.Controller.Flags.EnableGC = ToBool(env[ENABLE_GC])
	c.Controller.Flags.EnableGCAppRule = ToBool(env[ENABLE_GC_APP_RULE])
	c.Controller.Flags.EnablePrometheus = ToBool(env[ENABLE_PROMETHEUS])
	c.Controller.Flags.EnablePortProbe = ToBool(env[ENABLE_PORTPROBE])
	c.Controller.Flags.EnableIPV6 = ToBool(env[ENABLE_IPV6])
	c.Controller.Flags.EnableHTTP2 = ToBool(env[ENABLE_HTTP2])
	c.Controller.Flags.EnableCrossClusters = ToBool(env[SERVE_CROSSCLUSTERS])
	c.Controller.Flags.EnableGzip = ToBool(env[ENABLE_GZIP])
	c.Controller.Flags.EnableGoMonitor = ToBool(env[ENABLE_GO_MONITOR])
	c.Controller.Flags.EnableProfile = ToBool(env[ENABLE_PROFILE])
	c.Controller.Flags.EnableIngress = ToBool(env[SERVE_INGRESS])
	c.Controller.Flags.PolicyZip = ToBool(env[POLICY_ZIP])
	c.Controller.Flags.EnableLbSvc = ToBool(env[ENABLE_VIP])
	c.Controller.Flags.DebugRuleSync = ToBoolOr(env[DEBUG_RULESYNC], false)
}

func gatewayFromEnv(env map[string]string, a *ALBRunConfig) {
	a.Gateway.Enable = ToBool(env[GATEWAY_ENABLE])
	if !a.Gateway.Enable {
		return
	}
	mode := env[GATEWAY_MODE]
	if mode == string(GatewayModeStandAlone) {
		a.Gateway.Mode = GatewayModeStandAlone
		a.Gateway.StandAlone = &GatewayStandAloneConfig{
			env[GATEWAY_NAME],
			env[GATEWAY_NS],
		}
	}
	if mode == string(GatewayModeShared) {
		a.Gateway.Mode = GatewayModeShared
		a.Gateway.Shared = &GatewaySharedConfig{
			env[GATEWAY_CLASSNAME],
		}
	}
}

func IntToEnv(x int) string {
	return fmt.Sprintf("%d", x)
}
func BoolToEnv(x bool) string {
	return fmt.Sprintf("%v", x)
}

func ToInt(x string) int {
	i, err := strconv.Atoi(x)
	if err != nil {
		panic(fmt.Sprintf("invalid int env  %v %v", x, err))
	}
	return i
}

func ToIntOr(x string, backup int) int {
	if x == "" {
		return backup
	}
	i, err := strconv.Atoi(x)
	if err != nil {
		return backup
	}
	return i
}
func ToStringOr(x string, backup string) string {
	if x == "" {
		return backup
	}
	return x
}

func ToBoolOr(x string, backup bool) bool {
	if x == "" {
		return backup
	}
	return strings.ToLower(strings.TrimSpace(x)) == "true"
}

func ToBool(x string) bool {
	return strings.ToLower(strings.TrimSpace(x)) == "true"
}

func ToStrOr(x string, backup string) string {
	if x == "" {
		return backup
	}
	return x
}
