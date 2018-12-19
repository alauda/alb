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
		"LB_TYPE":               Haproxy,
		"SCHEDULER":             Kubernetes,
		"NEW_CONFIG_PATH":       "haproxy.cfg.new",
		"OLD_CONFIG_PATH":       "haproxy.cfg",
		"HAPROXY_TEMPLATE_PATH": "template/haproxy/haproxy.tmpl",
		"HAPROXY_BIN_PATH":      "haproxy",
		"NAME":                  "haproxy-test",
		"NAMESPACE":             "default",
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

func TestLabels(t *testing.T) {
	a := assert.New(t)
	Initialize()
	a.NotEmpty(Get("labels.name"))
	a.NotEmpty(Get("labels.frontend"))
}

func TestStandAlone(t *testing.T) {
	ENV := map[string]string{
		"LB_TYPE":               Haproxy,
		"SCHEDULER":             Kubernetes,
		"NEW_CONFIG_PATH":       "haproxy.cfg.new",
		"OLD_CONFIG_PATH":       "haproxy.cfg",
		"HAPROXY_TEMPLATE_PATH": "template/haproxy/haproxy.tmpl",
		"HAPROXY_BIN_PATH":      "haproxy",
		"NAMESPACE":             "default",
		"NAME":                  "haproxy-test",
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
	err := ValidateConfig()
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
