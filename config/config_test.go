package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func cleanenv(keys map[string]string) {
	for key := range keys {
		os.Setenv(key, "")
	}
}

func TestEnvConfig(t *testing.T) {
	a := assert.New(t)
	ENV := map[string]string{
		"LB_TYPE":             Nginx,
		"SCHEDULER":           Kubernetes,
		"NEW_CONFIG_PATH":     "new",
		"OLD_CONFIG_PATH":     "old",
		"NGINX_TEMPLATE_PATH": "tpl",
		"NEW_POLICY_PATH":     "new",
		"NAMESPACE":           "default",
		"NAME":                "ngx-test",
		"DOMAIN":              "alauda.io",
		"MODE":                "controller",
		"NETWORK_MODE":        "host",
		"MY_POD_NAME":         "p1",
	}
	for key, val := range ENV {
		os.Setenv(key, val)
	}
	Initialize()

	for key, val := range ENV {
		a.Equal(val, Get(key))
	}

	err := Init()
	a.Nil(err)

	cleanenv(ENV)
}

func TestLabels(t *testing.T) {
	a := assert.New(t)
	Initialize()
	a.NotEmpty(Get("labels.name"))
	a.NotEmpty(Get("labels.frontend"))
}

func TestStandAlone(t *testing.T) {
	ENV := map[string]string{
		"LB_TYPE":             Nginx,
		"SCHEDULER":           Kubernetes,
		"NEW_CONFIG_PATH":     "new",
		"OLD_CONFIG_PATH":     "old",
		"NGINX_TEMPLATE_PATH": "tpl",
		"MODE":                "controller",
		"NETWORK_MODE":        "host",
		"MY_POD_NAME":         "p1",
		"NEW_POLICY_PATH":     "new",
		"NAMESPACE":           "default",
		"NAME":                "ngx-test",
		"DOMAIN":              "alauda.io",
	}
	a := assert.New(t)
	a.True(IsStandalone())
	for key, val := range ENV {
		os.Setenv(key, val)
	}
	Initialize()

	os.Setenv("ALB_STANDALONE", "true")
	Initialize()
	a.True(IsStandalone())
	err := Init()
	a.Nil(err)

	cleanenv(ENV)
}

func TestDefault(t *testing.T) {
	a := assert.New(t)
	Initialize()

	a.Equal(30, GetInt("timeout"))
	os.Setenv("INTERVAL", "10")
	// read from env
	a.Equal(10, GetInt("INTERVAL"))
	os.Clearenv()
	// read from viper-config.toml
	a.Equal(5, GetInt("INTERVAL"))

	a.False(GetBool("RECORD_POST_BODY"))
	a.True(GetBool("USE_POD_HOST_IP"))
}

func TestConfig(t *testing.T) {
	Init()
	os.Setenv("ALB_LOCK_TIMEOUT", "100")
	ret := GetInt("LOCK_TIMEOUT")
	assert.Equal(t, ret, 100)
	os.Setenv("CPU_PRESET", "8")
	ret2 := GetInt("CPU_PRESET")
	assert.Equal(t, ret2, 8)
	os.Setenv("CPU_PRESET", "1600m")
	ret3 := GetInt("CPU_PRESET")
	assert.Equal(t, ret3, 2)

	Set("METRICS_PORT", "1936")
	Set("BACKLOG", "100")
	Set("MODE", "controller")
	Set("NETWORK_MODE", "host")
	Set("ALB_ENABLE", "true")
	assert.Equal(t, GetConfig().IsEnableAlb(), true)
	Set("ALB_ENABLE", "false")
	assert.Equal(t, GetConfig().IsEnableAlb(), false)

}
