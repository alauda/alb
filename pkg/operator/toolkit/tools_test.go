package toolkit

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestYamlToJson(t *testing.T) {
	y := `
antiAffinityKey: system
defaultSSLCert: cpaas-system/dex.tls
defaultSSLStrategy: Both
loadbalancerName: "global-alb2"
metricsPort: 11782
ingress: "true"
albEnable: false
projects:
    - cpaas-system
`
	j, err := YamlToJson(y)
	assert.NoError(t, err)
	t.Logf("%s", j)
	assert.Equal(t, j, `{"albEnable":false,"antiAffinityKey":"system","defaultSSLCert":"cpaas-system/dex.tls","defaultSSLStrategy":"Both","ingress":"true","loadbalancerName":"global-alb2","metricsPort":11782,"projects":["cpaas-system"]}`)
}
