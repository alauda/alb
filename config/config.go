package config

// 这里的config的目的是将环境变量或者默认配置文件转换为内存中的结构体，方便后续使用
// TODO pkg/operator/config负责的是将alb cr上的配置 转换为deployment的环境变量,两者应该合并起来
import (
	"fmt"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
	"github.com/thoas/go-funk"
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

var requiredFields = []string{
	"NAME",
	"NAMESPACE",
	"DOMAIN",
	"MY_POD_NAME",
	"MODE",
	"NETWORK_MODE",
}

// ALL ENV
// TODO use enum
var optionalFields = []string{
	"ALB_ENABLE",
	"NEW_NAMESPACE",
	"KUBERNETES_SERVER",
	"KUBERNETES_BEARERTOKEN",
	"SCHEDULER",
	"LB_TYPE",
	"KUBERNETES_TIMEOUT",
	"INTERVAL",
	//for xiaoying
	"RECORD_POST_BODY",
	// set to "true" if want to use nodes which pods run on them
	"USE_POD_HOST_IP",
	"ENABLE_GC",
	"ENABLE_GC_APP_RULE",
	"ENABLE_IPV6",
	"ENABLE_PROMETHEUS",
	"ENABLE_PROFILE",
	"ENABLE_PORTPROBE",
	"DEFAULT-SSL-CERTIFICATE",
	"DEFAULT-SSL-STRATEGY",
	"SERVE_INGRESS",
	"SERVE_CROSSCLUSTERS",
	"INGRESS_HTTP_PORT",
	"INGRESS_HTTPS_PORT",
	"ENABLE_HTTP2",
	"SCENARIO",
	"WORKER_LIMIT",
	"CPU_PRESET",
	"RESYNC_PERIOD",
	"ENABLE_GO_MONITOR",
	"GO_MONITOR_PORT",
	"METRICS_PORT",
	"BACKLOG",
	"ENABLE_GZIP",
	"POLICY_ZIP",
	// gateway related config
	"GATEWAY_ENABLE",
	"GATEWAY_MODE",
	"GATEWAY_NAME",

	// leader related config
	"LEADER_LEASE_DURATION",
	"LEADER_RENEW_DEADLINE",
	"LEADER_RETRY_PERIOD",

	"ENABLE_VIP",
}

var nginxRequiredFields = []string{
	"NEW_CONFIG_PATH",
	"OLD_CONFIG_PATH",
	"NGINX_TEMPLATE_PATH",
	"NEW_POLICY_PATH",
}

func initViper() {
	viper.SetConfigName("viper-config")
	viper.AddConfigPath(".")
	viper.AddConfigPath("../")
	viper.AddConfigPath("/alb/ctl")
	viper_base := os.Getenv("VIPER_BASE")
	if viper_base != "" {
		viper.AddConfigPath(viper_base)
	}
	viper.SetEnvPrefix("alb")
}

func setDefault() {
	defaultConfig := viper.GetStringMapString("default")
	for key, val := range defaultConfig {
		viper.SetDefault(key, val)
	}
}

// Initialize initializes the configuration
func Initialize() {
	viper.ReadInConfig()
	setDefault()
	getEnvs(requiredFields)
	getEnvs(optionalFields)
	getEnvs(nginxRequiredFields)
	viper.AutomaticEnv()
}

func getEnvs(fs []string) {
	for _, f := range fs {
		viper.BindEnv(f, f)
	}
}

func checkEmpty(requiredFields []string) []string {
	emptyRequiredEnv := []string{}
	for _, f := range requiredFields {
		if viper.GetString(f) == "" {
			emptyRequiredEnv = append(emptyRequiredEnv, f)
		}
	}
	return emptyRequiredEnv
}

func Init() error {
	initViper()
	Initialize()
	emptyRequiredEnv := checkEmpty(requiredFields)
	if len(emptyRequiredEnv) > 0 {
		return fmt.Errorf("%s env vars are requied but empty", strings.Join(emptyRequiredEnv, ","))
	}

	switch strings.ToLower(Get("LB_TYPE")) {
	case Nginx:
		emptyRequiredEnv = checkEmpty(nginxRequiredFields)
		if len(emptyRequiredEnv) > 0 {
			return fmt.Errorf("%s envvars are requied but empty", strings.Join(emptyRequiredEnv, ","))
		}
		if funk.ContainsString([]string{"Always", "Request", "Both"}, Get("DEFAULT-SSL-STRATEGY")) {
			if Get("DEFAULT-SSL-CERTIFICATE") == "" {
				return fmt.Errorf("no default ssl cert defined for nginx")
			}
		}
	default:
		return fmt.Errorf("Unsuported lb type %s", Get("LB_TYPE"))
	}

	return nil
}

// IsStandalone return true if alb is running in stand alone mode
func IsStandalone() bool {
	return true
}

// Set key to val
func Set(key, val string) {
	viper.Set(key, val)
}

// Get return string value of keyGet
func Get(key string) string {
	v := viper.GetString(key)
	return v
}

// GetBool return bool value of the key
func GetBool(key string) bool {
	v := viper.GetBool(key)
	return v
}

func SetBool(key string, val bool) {
	viper.Set(key, val)
}

// TODO dont handle cpu limit here. operator should do it.
// GetInt return int value of the key
func GetInt(key string) int {
	v := viper.GetString(key)
	// cpu limit could have value like 200m, need some calculation
	re := regexp.MustCompile(`([0-9]+)m`)
	var val int
	if string_decimal := strings.TrimRight(re.FindString(fmt.Sprintf("%v", v)), "m"); string_decimal == "" {
		val, _ = strconv.Atoi(v)
	} else {
		val_decimal, _ := strconv.Atoi(string_decimal)
		val = int(math.Ceil(float64(val_decimal) / 1000))
	}
	return val
}

func GetLabelSourceType() string {
	return fmt.Sprintf(Get("labels.source_type"), Get("DOMAIN"))
}

func GetLabelAlbName() string {
	return fmt.Sprintf(Get("labels.name"), Get("DOMAIN"))
}

func GetLabelSourceIngressHash() string {
	return fmt.Sprintf(Get("labels.source_name_hash"), Get("DOMAIN"))
}

func GetLabelSourceIngressVersion() string {
	return fmt.Sprintf(Get("labels.source_ingress_version"), Get("DOMAIN"))
}

// IngressClassConfiguration defines the various aspects of IngressClass parsing
// and how the controller should behave in each case
type IngressClassConfiguration struct {
	// Controller defines the controller value this daemon watch to.
	// Defaults to "alauda.io/alb2"
	Controller string
	// AnnotationValue defines the annotation value this Controller watch to, in case of the
	// ingressClass is not found but the annotation is.
	// The Annotation is deprecated and should not be used in future releases
	AnnotationValue string
	// WatchWithoutClass defines if Controller should watch to Ingress Objects that does
	// not contain an IngressClass configuration
	WatchWithoutClass bool
	// IgnoreIngressClass defines if Controller should ignore the IngressClass Object if no permissions are
	// granted on IngressClass
	IgnoreIngressClass bool
	//IngressClassByName defines if the Controller should watch for Ingress Classes by
	// .metadata.name together with .spec.Controller
	IngressClassByName bool
}

// a wrapper of common used config.getXX
type Config struct {
}

func (c *Config) GetNs() string {
	return Get("NAMESPACE")
}

func (c *Config) GetMetricsPort() int {
	return GetInt("METRICS_PORT")
}

func (c *Config) GetAlbName() string {
	return Get("NAME")
}

func (c *Config) GetDomain() string {
	return Get("DOMAIN")
}

func (c *Config) GetPodName() string {
	return Get("MY_POD_NAME")
}

func (c *Config) GetLabelLeader() string {
	return fmt.Sprintf(Get("labels.leader"), c.GetDomain())
}

func (c *Config) GetDefaultSSLSCert() string {
	return Get("DEFAULT-SSL-CERTIFICATE")
}

func (c *Config) GetDefaultSSLStrategy() string {
	return Get("DEFAULT-SSL-STRATEGY")
}

func (c *Config) GetIngressHttpPort() int {
	return GetInt("INGRESS_HTTP_PORT")
}

func (c *Config) GetIngressHttpsPort() int {
	return GetInt("INGRESS_HTTPS_PORT")
}

func (c *Config) GetLabelSourceIngressVer() string {
	sourceIngressVersion := fmt.Sprintf(Get("labels.source_ingress_version"), c.GetDomain())
	return sourceIngressVersion
}

func (c *Config) GetLabelSourceIngressPathIndex() string {
	return fmt.Sprintf(Get("labels.source_ingress_path_index"), c.GetDomain())
}

func (c *Config) GetLabelSourceIngressRuleIndex() string {
	return fmt.Sprintf(Get("labels.source_ingress_rule_index"), c.GetDomain())
}

const INGRESS_ADDRESS_NAME = "alb2.%s/%s_address"

func (c *Config) GetAnnotationIngressAddress() string {
	return fmt.Sprintf(INGRESS_ADDRESS_NAME, c.GetDomain(), c.GetAlbName())
}

func GetConfig() *Config {
	return &Config{}
}

// Deprecated: use GetConfig()
func GetAlbName() string {
	return Get("NAME")
}

// Deprecated: use GetConfig()
func GetNs() string {
	return Get("NAMESPACE")
}

// Deprecated: use GetConfig()
func GetDomain() string {
	return Get("DOMAIN")
}

func (c *Config) GetLeaderConfig() LeaderConfig {
	return LeaderConfig{
		LeaseDuration: time.Second * time.Duration(GetInt("LEADER_LEASE_DURATION")),
		RenewDeadline: time.Second * time.Duration(GetInt("LEADER_RENEW_DEADLINE")),
		RetryPeriod:   time.Second * time.Duration(GetInt("LEADER_RETRY_PERIOD")),
	}
}

func (c *Config) DebugRuleSync() bool {
	return os.Getenv("DEBUG_RULESYNC") == "true"
}

func (c *Config) GetLabelAlbName() string {
	return fmt.Sprintf(Get("labels.name"), c.GetDomain())
}

func (c *Config) GetLabelSourceType() string {
	return fmt.Sprintf(Get("labels.source_type"), c.GetDomain())
}

func (c *Config) GetNetworkMode() ControllerNetWorkMode {
	// TODO use map?
	mode := Get(NetworkModeKey)
	switch mode {
	case string(Host):
		return Host
	case string(Container):
		return Container
	}
	panic(fmt.Sprintf("invalid mode %v", mode))
}

func (c *Config) GetMode() Mode {
	// TODO use map?
	switch Get(ModeKey) {
	case string(Controller):
		return Controller
	case string(Operator):
		return Operator
	}
	panic("invalid mode")
}

// TODO a better name
func (c *Config) IsEnableAlb() bool {
	return GetBool("ALB_ENABLE")
}

func (c *Config) IsEnableVIP() bool {
	return GetBool("ENABLE_VIP")
}
