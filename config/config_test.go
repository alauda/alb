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

func TestConfigInit(t *testing.T) {
	a := assert.New(t)
	Initialize()
	err := ValidateConfig()
	a.Error(err)
}

func TestEnvConfig(t *testing.T) {
	a := assert.New(t)
	ENV := map[string]string{
		"LB_TYPE":                Haproxy,
		"SCHEDULER":              Kubernetes,
		"KUBERNETES_SERVER":      "http://127.0.0.1:6443",
		"KUBERNETES_BEARERTOKEN": "1234.567890",
		"NEW_CONFIG_PATH":        "haproxy.cfg.new",
		"OLD_CONFIG_PATH":        "haproxy.cfg",
		"HAPROXY_TEMPLATE_PATH":  "template/haproxy/haproxy.tmpl",
		"HAPROXY_BIN_PATH":       "haproxy",
		"NAME":                   "haproxy-test",
		"JAKIRO_ENDPOINT":        "http://127.0.0.1:8080",
		"TOKEN":                  "abcdef1234567890",
		"NAMESPACE":              "default",
		"REGION_NAME":            "test",
	}
	for key, val := range ENV {
		os.Setenv(key, val)
	}
	Initialize()

	for key, val := range ENV {
		a.Equal(val, Config[key])
		a.Equal(val, Get(key))
	}

	err := ValidateConfig()
	a.Nil(err)

	cleanenv(ENV)
}

func TestStandAlone(t *testing.T) {
	ENV := map[string]string{
		"LB_TYPE":                Haproxy,
		"SCHEDULER":              Kubernetes,
		"KUBERNETES_SERVER":      "http://127.0.0.1:6443",
		"KUBERNETES_BEARERTOKEN": "1234.567890",
		"NEW_CONFIG_PATH":        "haproxy.cfg.new",
		"OLD_CONFIG_PATH":        "haproxy.cfg",
		"HAPROXY_TEMPLATE_PATH":  "template/haproxy/haproxy.tmpl",
		"HAPROXY_BIN_PATH":       "haproxy",
		"NAME":                   "haproxy-test",
	}
	a := assert.New(t)
	a.False(IsStandalone())
	for key, val := range ENV {
		os.Setenv(key, val)
	}
	Initialize()

	err := ValidateConfig()
	a.Error(err)

	os.Setenv("ALB_STANDALONE", "true")
	Initialize()
	a.True(IsStandalone())
	err = ValidateConfig()
	a.Nil(err)

	cleanenv(ENV)
}

func TestDefault(t *testing.T) {
	a := assert.New(t)
	Initialize()

	a.Equal(30, GetInt("timeout"))
	a.Equal(10, GetInt("INTERVAL"))

	a.Equal("/alb/certificates", Get("CERTIFICATE_DIRECTORY"))

	a.False(GetBool("RECORD_POST_BODY"))
	a.True(GetBool("USE_POD_HOST_IP"))
}
