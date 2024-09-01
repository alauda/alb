package config

// 这里的 config 的目的是将环境变量或者默认配置文件转换为内存中的结构体，方便后续使用
import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	. "alauda.io/alb2/pkg/config"
	"alauda.io/alb2/utils"
	"alauda.io/alb2/utils/log"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	Kubernetes = "kubernetes"

	Nginx = "nginx"

	// IngressKey picks a specific "class" for the Ingress.
	// The controller only processes Ingresses with this annotation either
	// unset, or set to either the configured value or the empty string.
	IngressKey = "kubernetes.io/ingress.class"

	// DefaultControllerName defines the default controller name for Ingress controller alb2
	DefaultControllerName = "alb2"
)

type K8sConfig struct {
	Mode      string // test | kubecfg | kube_xx | incluster
	KubeCfg   string
	K8sServer string
	K8sToken  string
}

type ExtraConfig struct {
	Pod                         string
	ReloadNginx                 bool
	FullSync                    bool
	DisablePeriodGenNginxConfig bool
	E2eTestControllerOnly       bool

	Interval             int
	ReloadTimeout        int
	DebugRuleSync        bool
	NginxTemplatePath    string
	NewConfigPath        string
	OldConfigPath        string
	NewPolicyPath        string
	TweakDir             string
	K8s                  K8sConfig
	StatusFileParentPath string
	Leader               LeaderConfig
}

type LeaderConfig struct {
	LeaseDuration time.Duration
	RenewDeadline time.Duration
	RetryPeriod   time.Duration
	SkipExit      bool
}

func K8sFromEnv() K8sConfig {
	return k8sFromEnv(getAllEnv())
}

func k8sFromEnv(env map[string]string) K8sConfig {
	origin := K8sConfig{
		Mode: "incluster",
	}
	if env[USE_KUBE_CONFIG] != "" {
		origin.Mode = "kubecfg"
		origin.KubeCfg = env[USE_KUBE_CONFIG]
	}
	if env[KUBE_SERVER] != "" {
		origin.Mode = "kube_xx"
		origin.K8sServer = env[KUBE_SERVER]
		origin.K8sToken = env[KUBE_TOKEN]
	}
	return origin
}

func ExtraFlagsFromEnv(env map[string]string) ExtraConfig {
	return ExtraConfig{
		ReloadNginx:                 ToBoolOr(env[RELOAD_NGINX], true),
		DisablePeriodGenNginxConfig: ToBoolOr(env[DISABLE_PERIOD_GEN_NGINX_CONFIG], false),
		E2eTestControllerOnly:       ToBoolOr(env[E2E_TEST_CONTROLLER_ONLY], false),
		FullSync:                    ToBoolOr(env[FULL_SYNC], true),
		NginxTemplatePath:           ToStrOr(env[NGINX_TEMPLATE_PATH], NGINX_TEMPLATE_PATH_VAL),
		NewConfigPath:               ToStrOr(env[NEW_CONFIG_PATH], NEW_CONFIG_PATH_VAL),
		OldConfigPath:               ToStrOr(env[OLD_CONFIG_PATH], OLD_CONFIG_PATH_VAL),
		NewPolicyPath:               ToStrOr(env[NEW_POLICY_PATH], NEW_POLICY_PATH_VAL),
		TweakDir:                    TWEAK_DIR_VAL,
		Interval:                    ToIntOr(env[INTERVAL], INTERVAL_VAL),
		ReloadTimeout:               ToIntOr(env[RELOAD_TIMEOUT], DEFAULT_RELOAD_TIMEOUT_VAL),
		Pod:                         env[MY_POD_NAME],
		DebugRuleSync:               ToBoolOr(env[DEBUG_RULESYNC], false),
		K8s:                         k8sFromEnv(env),
		StatusFileParentPath:        STATUS_FILE_PARENT_PATH_VAL,
		Leader: LeaderConfig{
			LeaseDuration: time.Second * time.Duration(120),
			RenewDeadline: time.Second * time.Duration(40),
			RetryPeriod:   time.Second * time.Duration(12),
		},
	}
}

type Config struct {
	ALBRunConfig
	ExtraConfig
	Names
}

var (
	cfg  *Config
	once sync.Once
)

func getAllEnv() map[string]string {
	e := map[string]string{}
	for _, kv := range os.Environ() {
		kvs := strings.Split(kv, "=")
		e[kvs[0]] = kvs[1]
	}
	return e
}

func InitFromEnv(env map[string]string) *Config {
	for k, v := range env {
		log.L().Info("all env", "key", k, "val", v)
	}
	acfg, err := AlbRunCfgFromEnv(env)
	if err != nil {
		panic(err)
	}
	cfg := &Config{
		ALBRunConfig: acfg,
		ExtraConfig:  ExtraFlagsFromEnv(env),
		Names:        Names{domain: acfg.Domain},
	}
	log.L().Info("alb cfg from env", "cfg", utils.PrettyJson(cfg))
	return cfg
}

func InTestSetConfig(c Config) {
	cfg = &c
	log.L().Info("init test mode cfg", "cfg", utils.PrettyJson(GetConfig()))
}

func GetConfig() *Config {
	once.Do(func() {
		if cfg != nil {
			return
		}
		env := getAllEnv()
		cfg = InitFromEnv(env)
	})
	return cfg
}

func (c *Config) SetDomain(domain string) {
	c.domain = domain
	c.Domain = domain
}

func (c *Config) GetNs() string {
	return c.Ns
}

func (c *Config) GeKey() client.ObjectKey {
	return client.ObjectKey{Namespace: c.Ns, Name: c.Name}
}

func (c *Config) GetMetricsPort() int {
	return c.Controller.MetricsPort
}

func (c *Config) GetInterval() int {
	return c.ExtraConfig.Interval
}

func (c *Config) GetReloadTimeout() int {
	return c.ExtraConfig.ReloadTimeout
}

func (c *Config) GetResyncPeriod() int {
	return c.Controller.ResyncPeriod
}

func (c *Config) GetAlbName() string {
	return c.Name
}

func (c *Config) GetAlbNsAndName() (string, string) {
	return c.GetNs(), c.GetAlbName()
}

func (c *Config) GetDomain() string {
	return c.Domain
}

func (c *Config) GetPodName() string {
	return c.ExtraConfig.Pod
}

const (
	FMT_BINDKEY                   = "loadbalancer.%s/bind"
	FMT_NAME                      = "alb2.%s/name"
	FMT_FT                        = "alb2.%s/frontend"
	FMT_LEADER                    = "alb2.%s/leader"
	FMT_SOURCE_TYPE               = "alb2.%s/source-type"
	FMT_SOURCE_NAME               = "alb2.%s/source-name"
	FMT_SOURCE_NS                 = "alb2.%s/source-ns"
	FMT_SOURCE_INDEX              = "alb2.%s/source-index"
	FMT_PROJECT                   = "%s/project"
	FMT_SOURCE_INGRESS_RULE_INDEX = "alb2.%s/source-ingress-rule-index"
	FMT_SOURCE_INGRESS_PATH_INDEX = "alb2.%s/source-ingress-path-index"
	FMT_INGRESS_ADDRESS_NAME      = "alb2.%s/%s_address"
	OVERWRITE_CONFIGMAP           = "alb2.operator.%s/overwrite-configmap"
)

type Names struct {
	domain string
}

func NewNames(domain string) Names {
	return Names{domain: domain}
}

func (n Names) GetLabelLeader() string {
	return fmt.Sprintf(FMT_LEADER, n.domain)
}

func (n Names) GetLabelSourceIngressPathIndex() string {
	return fmt.Sprintf(FMT_SOURCE_INGRESS_PATH_INDEX, n.domain)
}

func (n Names) GetLabelBindKey() string {
	return fmt.Sprintf(FMT_BINDKEY, n.domain)
}

func (n Names) GetLabelSourceIngressRuleIndex() string {
	return fmt.Sprintf(FMT_SOURCE_INGRESS_RULE_INDEX, n.domain)
}

func (n Names) GetLabelAlbName() string {
	return fmt.Sprintf(FMT_NAME, n.domain)
}

func (n Names) GetLabelFt() string {
	return fmt.Sprintf(FMT_FT, n.domain)
}

func (n Names) GetLabelSourceType() string {
	return fmt.Sprintf(FMT_SOURCE_TYPE, n.domain)
}

func (n Names) GetLabelSourceIndex() string {
	return fmt.Sprintf(FMT_SOURCE_INDEX, n.domain)
}
func (n Names) GetLabelSourceName() string {
	return fmt.Sprintf(FMT_SOURCE_NAME, n.domain)
}

func (n Names) GetLabelSourceNs() string {
	return fmt.Sprintf(FMT_SOURCE_NS, n.domain)
}

func (n Names) GetLabelProject() string {
	return fmt.Sprintf(FMT_PROJECT, n.domain)
}

func (n Names) GetOverwriteConfigmapLabelKey() string {
	return fmt.Sprintf(OVERWRITE_CONFIGMAP, n.domain)
}

func (n Names) GetAlbIngressRewriteResponseAnnotation() string {
	return fmt.Sprintf("alb.ingress.%s/rewrite-response", n.domain)
}

func (n Names) GetAlbIngressOtelAnnotation() string {
	return fmt.Sprintf("alb.ingress.%s/otel", n.domain)
}

func (n Names) GetAlbRuleRewriteResponseAnnotation() string {
	return fmt.Sprintf("alb.rule.%s/rewrite-response", n.domain)
}

func (n Names) GetAlbIngressRewriteRequestAnnotation() string {
	return fmt.Sprintf("alb.ingress.%s/rewrite-request", n.domain)
}

func (n Names) GetAlbRuleRewriteRequestAnnotation() string {
	return fmt.Sprintf("alb.rule.%s/rewrite-request", n.domain)
}

type Flags struct {
	ControllerFlags
	ExtraConfig
}

func (c *Config) GetFlags() Flags {
	return Flags{
		ControllerFlags: c.Controller.Flags,
		ExtraConfig:     c.ExtraConfig,
	}
}

func (c *Config) GetStatusFile() string {
	return c.StatusFileParentPath
}

func (c *Config) GetDefaultSSLCert() string {
	return c.Controller.SSLCert
}

func (c *Config) GetDefaultSSLStrategy() string {
	return c.Controller.DefaultSSLStrategy
}

func (c *Config) GetIngressHttpPort() int {
	return c.Controller.HttpPort
}

func (c *Config) GetIngressHttpsPort() int {
	return c.Controller.HttpsPort
}

func (c *Config) GetCpuPreset() int {
	return c.Controller.CpuPreset
}

func (c *Config) GetWorkerLimit() int {
	return c.Controller.WorkerLimit
}

func (c *Config) GetLeaderConfig() LeaderConfig {
	return c.ExtraConfig.Leader
}

func (c *Config) DebugRuleSync() bool {
	return c.ExtraConfig.DebugRuleSync
}

func (c *Config) EnableIngress() bool {
	return c.Controller.Flags.EnableIngress
}

func (c *Config) GetNetworkMode() ControllerNetWorkMode {
	return ControllerNetWorkMode(c.Controller.NetworkMode)
}

// TODO a better name
func (c *Config) IsEnableAlb() bool {
	return c.Controller.Flags.EnableAlb
}

func (c *Config) IsEnableVIP() bool {
	return c.Controller.Flags.EnableLbSvc
}

func (c *Config) GetGoMonitorPort() int {
	return c.Controller.GoMonitorPort
}

func (c *Config) GetNginxCfg() NginxCfg {
	return NginxCfg{
		NginxTemplatePath: c.ExtraConfig.NginxTemplatePath,
		NewConfigPath:     c.ExtraConfig.NewConfigPath,
		OldConfigPath:     c.ExtraConfig.OldConfigPath,
		NewPolicyPath:     c.ExtraConfig.NewPolicyPath,
		EnablePrometheus:  c.Controller.Flags.EnablePrometheus,
		EnableHttp2:       c.Controller.Flags.EnableHTTP2,
		EnableGzip:        c.Controller.Flags.EnableGzip,
		BackLog:           c.Controller.BackLog,
		EnableIpv6:        c.Controller.Flags.EnableIPV6,
		TweakDir:          c.ExtraConfig.TweakDir,
	}
}
