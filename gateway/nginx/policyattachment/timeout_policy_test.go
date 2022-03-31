package policyattachment

import (
	"encoding/json"
	"testing"

	gatewayPolicy "alauda.io/alb2/pkg/apis/alauda/gateway/v1alpha1"
	"github.com/openlyinc/pointy"
	"github.com/stretchr/testify/assert"
)

func TestIPolicyAttachmentConfig(t *testing.T) {
	timeout := gatewayPolicy.TimeoutPolicyConfig{
		ProxyConnectTimeoutMs: nil,
		ProxyReadTimeoutMs:    pointy.Uint(10),
	}
	timeoutJson, err := json.Marshal(timeout)
	assert.Equal(t, string(timeoutJson), `{"proxy_read_timeout_ms":10}`)
	assert.NoError(t, err)
	t.Logf("%s", timeoutJson)
	timeout1 := TimeoutPolicyConfig(timeout)
	ret := TimeoutPolicyConfig{}
	ret.FromConfig(timeout1.IntoConfig())
	assert.Nil(t, ret.ProxyConnectTimeoutMs)
	assert.Equal(t, *ret.ProxyReadTimeoutMs, uint(10))
	assert.Nil(t, ret.ProxySendTimeoutMs)
}
