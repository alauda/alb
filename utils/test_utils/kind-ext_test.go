package test_utils

import (
	"testing"

	"alauda.io/alb2/utils/log"
	"github.com/stretchr/testify/assert"
)

func TestGetConfig(t *testing.T) {
	t.Skip("dev")
	c := KindConfig{
		Name:  "alb-dual",
		Image: "kindest/node:v1.24.3",
		ClusterYaml: `
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
networking:
  ipFamily: dual
  apiServerAddress: "127.0.0.1"
nodes:
- role: control-plane
- role: worker
- role: worker 
`,
	}
	l := log.L()

	base := InitBase()
	kd, err := DeployOrAdopt(c, base, "alb-dual", l)
	assert.NoError(t, err)
	cfg, err := kd.GetConfig()
	assert.NoError(t, err)
	k := NewKubectl(base, cfg, l)
	out, err := k.Kubectl("get", "nodes")
	t.Log(out)
	assert.NoError(t, err)

}
