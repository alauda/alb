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
		"MY_POD_NAME":         "abc-xx",
		"DOMAIN":              "alauda.io",
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
		"MY_POD_NAME":         "abc-xx",
		"NEW_CONFIG_PATH":     "new",
		"OLD_CONFIG_PATH":     "old",
		"NGINX_TEMPLATE_PATH": "tpl",
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
	CleanCache()
	// read from viper-config.toml
	a.Equal(5, GetInt("INTERVAL"))
	a.Equal("/alb/certificates", Get("CERTIFICATE_DIRECTORY"))

	a.False(GetBool("RECORD_POST_BODY"))
	a.True(GetBool("USE_POD_HOST_IP"))
}

func TestConfig(t *testing.T) {
	os.Setenv("MY_POD_NAME", "abc-xx")
	os.Setenv("CPU_LIMIT", "8")
	Init()
	t.Logf("get env %v\n", os.Getenv("CPU_LIMIT"))
	ret2 := GetInt("CPU_LIMIT")
	assert.Equal(t, ret2, 8)
	CleanCache()
	os.Setenv("ALB_CPU_LIMIT", "1600m")
	ret3 := GetInt("CPU_LIMIT")
	t.Logf("ret 3 %v\n", ret3)
	assert.Equal(t, ret3, 2)

	ret4 := Get("MY_POD_NAME")
	t.Logf("ret 4 %v\n", ret4)
	assert.Equal(t, ret4, "abc-xx")
}
