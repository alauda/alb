package config

import (
	"fmt"
	"github.com/spf13/viper"
	"github.com/thoas/go-funk"
	"strings"
	"sync"
)

const (
	Kubernetes = "kubernetes"
)

const (
	SLB   = "slb"
	ELB   = "elb"
	CLB   = "clb"
	Nginx = "nginx"
)

var ConfigString sync.Map
var ConfigBool sync.Map
var ConfigInt sync.Map

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
	//USE_ENDPOINT MUST set to "true" if enable session affinity
	"USE_ENDPOINT",
	// set to "true" if want to use nodes which pods run on them
	"USE_POD_HOST_IP",
	"MY_POD_NAME",
	"ROTATE_INTERVAL",
	"ENABLE_GC",
	"ENABLE_IPV6",
	"ENABLE_PROMETHEUS",
	"ENABLE_PORTPROBE",
	"DEFAULT-SSL-CERTIFICATE",
	"DEFAULT-SSL-STRATEGY",
	"SERVE_INGRESS",
	"INGRESS_HTTP_PORT",
	"INGRESS_HTTPS_PORT",
}

var nginxRequiredFields = []string{
	"NEW_CONFIG_PATH",
	"OLD_CONFIG_PATH",
	"NGINX_TEMPLATE_PATH",
	"NEW_POLICY_PATH",
}

var cloudLBRequiredFields = []string{
	"IAAS_REGION",
	"ACCESS_KEY",
	"SECRET_ACCESS_KEY"}

func init() {
	initViper()
	Initialize()
}

func initViper() {
	viper.SetConfigName("alb-config")
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
	getEnvs(cloudLBRequiredFields)
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
	case ELB, SLB, CLB:
		emptyRequiredEnv = checkEmpty(cloudLBRequiredFields)
		if len(emptyRequiredEnv) > 0 {
			return fmt.Errorf("%s envvars are requied but empty", strings.Join(emptyRequiredEnv, ","))
		}

		Set("USE_ENDPOINT", "")
		if Get("NAME") == "" {
			Set("NAME", "alb-xlb")
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

//GetInt reuturn int value of the key
func GetInt(key string) int {
	if val, ok := ConfigInt.Load(key); ok {
		return val.(int)
	}
	v := viper.GetInt(key)
	ConfigInt.Store(key, v)
	return v
}
