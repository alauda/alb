package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

const (
	Marathon   = "marathon"
	Kubernetes = "kubernetes"
)

const (
	SLB     = "slb"
	ELB     = "elb"
	CLB     = "clb"
	Haproxy = "haproxy"
	Nginx   = "nginx"
)

var Config = map[string]string{}

var requiredFields = []string{
	"LB_TYPE",
	"SCHEDULER",
}

var optionalFields = []string{
	"MARATHON_USERNAME",
	"MARATHON_PASSWORD",
	"MARATHON_TIMEOUT",
	"KUBERNETES_TIMEOUT",
	"INTERVAL",
	"CERTIFICATE_DIRECTORY",
	//for xiaoying
	"RECORD_POST_BODY",
	//USE_ENDPOINT MUST set to "true" if enable session affinity
	"USE_ENDPOINT",
	// set to "true" if want to use nodes which pods run on them
	"USE_POD_HOST_IP",
}

var nonStandAloneRequiredFields = []string{
	"JAKIRO_ENDPOINT",
	"TOKEN",
	"NAMESPACE",
	"REGION_NAME",
}

var marathonRequiredFields = []string{
	"MARATHON_SERVER",
}

var kubernetesRequiredFields = []string{
	"KUBERNETES_SERVER",
	"KUBERNETES_BEARERTOKEN",
}

var haproxyRequiredFields = []string{
	"NEW_CONFIG_PATH",
	"OLD_CONFIG_PATH",
	"HAPROXY_TEMPLATE_PATH",
	"HAPROXY_BIN_PATH",
	"NAME"}

var nginxRequiredFields = []string{
	"NEW_CONFIG_PATH",
	"OLD_CONFIG_PATH",
	"NGINX_TEMPLATE_PATH",
	"NGINX_BIN_PATH",
	"NEW_POLICY_PATH",
	"OLD_POLICY_PATH",
	"NAME"}

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
	getEnvs(nonStandAloneRequiredFields)
	getEnvs(marathonRequiredFields)
	getEnvs(kubernetesRequiredFields)
	getEnvs(haproxyRequiredFields)
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
		return fmt.Errorf("%s envvars are requied but empty", strings.Join(emptyRequiredEnv, ","))
	}

	switch strings.ToLower(Config["SCHEDULER"]) {
	case Kubernetes:
		emptyRequiredEnv := checkEmpty(kubernetesRequiredFields)
		if len(emptyRequiredEnv) > 0 {
			return fmt.Errorf("%s envvars are requied but empty", strings.Join(emptyRequiredEnv, ","))
		}
	case Marathon:
		emptyRequiredEnv := checkEmpty(marathonRequiredFields)
		if len(emptyRequiredEnv) > 0 {
			return fmt.Errorf("%s envvars are requied but empty", strings.Join(emptyRequiredEnv, ","))
		}
	default:
		return fmt.Errorf("Unsuported scheduler %s", Config["SCHEDULER"])
	}

	switch strings.ToLower(Config["LB_TYPE"]) {
	case Haproxy:
		emptyRequiredEnv = checkEmpty(haproxyRequiredFields)
		if len(emptyRequiredEnv) > 0 {
			return fmt.Errorf("%s envvars are requied but empty", strings.Join(emptyRequiredEnv, ","))
		}
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

	if !IsStandalone() {
		emptyRequiredEnv = checkEmpty(nonStandAloneRequiredFields)
		if len(emptyRequiredEnv) > 0 {
			return fmt.Errorf("%s envvars are requied but empty", strings.Join(emptyRequiredEnv, ","))
		}
	}
	return nil
}

// IsStandalone return true if alb is running in stand alone mode
func IsStandalone() bool {
	return GetBool("standalone")
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

//InitLabels setup labels base on the type of cluster
func InitLabels() {
	oldStyleLabels := map[string]string{
		"LABEL_SERVICE_ID":   "alauda_service_id",
		"LABEL_SERVICE_NAME": "service_name",
		"LABEL_CREATOR":      "alauda_owner",
	}
	newStyleLabels := map[string]string{
		"LABEL_SERVICE_ID":   "service.alauda.io/uuid",
		"LABEL_SERVICE_NAME": "service.alauda.io/name",
		"LABEL_CREATOR":      "service.alauda.io/createby",
	}
	var labelConfig map[string]string
	if IsStandalone() || GetBool("k8s_v3") {
		labelConfig = newStyleLabels
	} else {
		labelConfig = oldStyleLabels
	}
	for key, val := range labelConfig {
		Set(key, val)
	}
}
