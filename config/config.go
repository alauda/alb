package config

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"alauda.io/alb2/utils"

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

var ConfigString sync.Map
var ConfigBool sync.Map
var ConfigInt sync.Map

func CleanCache() {
	utils.CleanSyncMap(ConfigBool)
	utils.CleanSyncMap(ConfigString)
	utils.CleanSyncMap(ConfigInt)
}

var requiredFields = []string{
	"NAME",
	"NAMESPACE",
	"DOMAIN",
}

var optionalFields = []string{
	"NEW_NAMESPACE",
	"KUBERNETES_SERVER",
	"KUBERNETES_BEARERTOKEN",
	"SCHEDULER",
	"LB_TYPE",
	"KUBERNETES_TIMEOUT",
	"INTERVAL",
	"CERTIFICATE_DIRECTORY",
	"NGINX_BIN_PATH",
	//for xiaoying
	"RECORD_POST_BODY",
	// set to "true" if want to use nodes which pods run on them
	"USE_POD_HOST_IP",
	"MY_POD_NAME",
	"ENABLE_GC",
	"ENABLE_GC_APP_RULE",
	"ENABLE_IPV6",
	"ENABLE_PROMETHEUS",
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
	"CPU_LIMIT",
	"RESYNC_PERIOD",
	"ENABLE_PROFILE",
	"METRICS_PORT",
	"BACKLOG",
	"ENABLE_GZIP",

	// gateway related config
	"ENABLE_GATEWAY",
}

var nginxRequiredFields = []string{
	"NEW_CONFIG_PATH",
	"OLD_CONFIG_PATH",
	"NGINX_TEMPLATE_PATH",
	"NEW_POLICY_PATH",
}

func init() {
	initViper()
	Initialize()
}

func initViper() {
	viper.SetConfigName("viper-config")
	viper.AddConfigPath(".")
	viper.AddConfigPath("../")
	viper.AddConfigPath("/alb/")

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

func ValidateConfig() error {
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
	ConfigString.Store(key, val)
	viper.Set(key, val)
}

// Get return string value of keyGet
func Get(key string) string {
	if val, ok := ConfigString.Load(key); ok {
		return val.(string)
	}
	v := viper.GetString(key)
	ConfigString.Store(key, v)
	return v
}

//GetBool return bool value of the key
func GetBool(key string) bool {
	if val, ok := ConfigBool.Load(key); ok {
		return val.(bool)
	}
	v := viper.GetBool(key)
	ConfigBool.Store(key, v)
	return v
}

func SetBool(key string, val bool) {
	ConfigBool.Store(key, val)
	viper.Set(key, val)
}

//GetInt return int value of the key
func GetInt(key string) int {
	if val, ok := ConfigInt.Load(key); ok {
		return val.(int)
	}
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
	ConfigInt.Store(key, val)
	return val
}

func GetLabelSourceType() string {
	return fmt.Sprintf(Get("labels.source_type"), Get("DOMAIN"))
}

func GetAlbName() string {
	return Get("NAME")
}

func GetNs() string {
	return Get("NAMESPACE")
}

func GetDomain() string {
	return Get("DOMAIN")
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
