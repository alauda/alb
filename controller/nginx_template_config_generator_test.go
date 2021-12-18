package controller

import (
	"alauda.io/alb2/config"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestGetCurrentNetwork(t *testing.T) {
	_, err := GetCurrentNetwork()
	assert.NoError(t, err)
}

func TestBindIp(t *testing.T) {
	config.Set("EnableIPV6", "true")
	v4, v6, err := getBindIp(
		BindNICConfig{Nic: []string{"eth0"}},
		NetWorkInfo{"eth0": InterfaceInfo{
			"eth0",
			[]string{},
			[]string{"fe80::1", "fa80::1"},
		}})
	assert.NoError(t, err)
	assert.Equal(t, v4[0], "0.0.0.0")
	assert.Equal(t, v6[0], "[fa80::1]")

	v4, v6, err = getBindIp(
		BindNICConfig{Nic: []string{"eth0"}},
		NetWorkInfo{"eth0": InterfaceInfo{
			"eth0",
			[]string{},
			[]string{},
		}})
	assert.NoError(t, err)
	assert.Equal(t, v4[0], "0.0.0.0")
	assert.Equal(t, v6[0], "[::]")

	v4, v6, err = getBindIp(
		BindNICConfig{Nic: []string{}},
		NetWorkInfo{"eth0": InterfaceInfo{
			"eth0",
			[]string{},
			[]string{},
		}})
	assert.NoError(t, err)
	assert.Equal(t, v4[0], "0.0.0.0")
	assert.Equal(t, v6[0], "[::]")

	config.SetBool("EnableIPV6", false)
	v4, v6, err = getBindIp(
		BindNICConfig{Nic: []string{"eth0"}},
		NetWorkInfo{"eth0": InterfaceInfo{
			"eth0",
			[]string{},
			[]string{},
		}})
	assert.NoError(t, err)
	assert.Equal(t, v4[0], "0.0.0.0")
	assert.Equal(t, len(v6), 0)
}

func setBindIpConfig(configStr *string) {
	if configStr != nil {
		dir := os.TempDir()
		path := filepath.Join(dir, "bind_nic.json")
		config.Set("TWEAK_DIRECTORY", dir)
		ioutil.WriteFile(path, []byte(*configStr), 0666)
	}
}

func TestGetBindIpConfig(t *testing.T) {

	testGet := func(configStr *string) (BindNICConfig, error) {
		setBindIpConfig(configStr)
		return GetBindNICConfig()
	}
	cfg, err := testGet(nil)
	assert.NoError(t, err)
	assert.Equal(t, len(cfg.Nic), 0)

	configStr := `{"nic":["eth1"]}`
	cfg, err = testGet(&configStr)
	assert.NoError(t, err)
	assert.Equal(t, cfg.Nic[0], "eth1")

	configStr = `{}`
	cfg, err = testGet(&configStr)
	assert.NoError(t, err)
	assert.Equal(t, len(cfg.Nic), 0)

	configStr = ``
	cfg, err = testGet(&configStr)
	assert.NoError(t, err)
	assert.Equal(t, len(cfg.Nic), 0)

	configStr = `{"wrongjson`
	cfg, err = testGet(&configStr)
	assert.NotNil(t, err)
}