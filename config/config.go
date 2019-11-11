package config

import (
	"fmt"
	"github.com/spf13/viper"
	"strings"
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

var Config = map[string]string{}

var requiredFields = []string{
	"NAME",
	"NAMESPACE",
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
	"ENABLE_GC",
	"ROTATE_INTERVAL",
}

var nginxRequiredFields = []string{
	"NEW_CONFIG_PATH",
	"OLD_CONFIG_PATH",
	"NGINX_TEMPLATE_PATH",
	"NEW_POLICY_PATH",
	"OLD_POLICY_PATH",
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
		Config[key] = val
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
		Config[f] = viper.GetString(f)
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

	switch strings.ToLower(Config["LB_TYPE"]) {
	case Nginx:
		emptyRequiredEnv = checkEmpty(nginxRequiredFields)
		if len(emptyRequiredEnv) > 0 {
			return fmt.Errorf("%s envvars are requied but empty", strings.Join(emptyRequiredEnv, ","))
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
		return fmt.Errorf("Unsuported lb type %s", Config["LB_TYPE"])
	}

	return nil
}

// IsStandalone return true if alb is running in stand alone mode
func IsStandalone() bool {
	return true
}

// Set key to val
func Set(key, val string) {
	Config[key] = val
	viper.Set(key, val)
}

// Get return string value of keyGet
func Get(key string) string {
	return viper.GetString(key)
}

//GetBool return bool value of the key
func GetBool(key string) bool {
	return viper.GetBool(key)
}

//GetInt reuturn int value of the key
func GetInt(key string) int {
	return viper.GetInt(key)
}
