package ngxconf

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetCurrentNetwork(t *testing.T) {
	_, err := GetCurrentNetwork()
	assert.NoError(t, err)
}

func TestBindIp(t *testing.T) {
	v4, v6, err := getBindIp(
		BindNICConfig{Nic: []string{"eth0"}},
		NetWorkInfo{"eth0": InterfaceInfo{
			"eth0",
			[]string{},
			[]string{"fe80::1", "fa80::1"},
		}}, true)
	assert.NoError(t, err)
	assert.Equal(t, v4[0], "0.0.0.0")
	assert.Equal(t, v6[0], "[fa80::1]")

	v4, v6, err = getBindIp(
		BindNICConfig{Nic: []string{"eth0"}},
		NetWorkInfo{"eth0": InterfaceInfo{
			"eth0",
			[]string{},
			[]string{},
		}}, true)
	assert.NoError(t, err)
	assert.Equal(t, v4[0], "0.0.0.0")
	assert.Equal(t, v6[0], "[::]")

	v4, v6, err = getBindIp(
		BindNICConfig{Nic: []string{}},
		NetWorkInfo{"eth0": InterfaceInfo{
			"eth0",
			[]string{},
			[]string{},
		}}, true)
	assert.NoError(t, err)
	assert.Equal(t, v4[0], "0.0.0.0")
	assert.Equal(t, v6[0], "[::]")
	v4, v6, err = getBindIp(
		BindNICConfig{Nic: []string{"eth0"}},
		NetWorkInfo{"eth0": InterfaceInfo{
			"eth0",
			[]string{},
			[]string{},
		}}, false)
	assert.NoError(t, err)
	assert.Equal(t, v4[0], "0.0.0.0")
	assert.Equal(t, len(v6), 0)

	v4, _, err = getBindIp(
		BindNICConfig{Nic: []string{"eth0", "eth1"}},
		NetWorkInfo{
			"eth0": InterfaceInfo{
				"eth0",
				[]string{"192.168.1.1"},
				[]string{},
			},
			"eth1": InterfaceInfo{
				"eth1",
				[]string{"192.168.1.1"},
				[]string{},
			},
		}, false)
	assert.NoError(t, err)
	assert.Equal(t, v4[0], "192.168.1.1")
	assert.Equal(t, len(v4), 1)

	v4, v6, err = getBindIp(
		BindNICConfig{Nic: []string{"eth0"}},
		NetWorkInfo{"eth0": InterfaceInfo{
			"eth0",
			[]string{"192.168.0.2", "192.168.0.1"},
			[]string{},
		}}, false)
	assert.NoError(t, err)
	assert.Equal(t, len(v6), 0)
	// must be sorted,otherwise it will reload nginx every time.
	assert.Equal(t, v4[0], "192.168.0.1")
	assert.Equal(t, v4[1], "192.168.0.2")
	assert.Equal(t, len(v4), 2)
}

func TestGetBindIpConfig(t *testing.T) {
	testGet := func(configStr *string) (BindNICConfig, error) {
		if configStr == nil {
			return GetBindNICConfig("")
		}
		dir := os.TempDir()
		path := filepath.Join(dir, "bind_nic.json")
		ioutil.WriteFile(path, []byte(*configStr), 0o666)
		return GetBindNICConfig(dir)
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
