package helper

import (
	"testing"

	"alauda.io/alb2/utils/log"
	. "alauda.io/alb2/utils/test_utils"
	"github.com/stretchr/testify/assert"
)

func TestEchoResty(t *testing.T) {
	print(EchoRestyTemplate)
	base := InitBase()
	kd := AdoptKind(base, "alb-dual", log.L())
	cfg, err := kd.GetConfig()
	assert.NoError(t, err)
	image := "registry.alauda.cn:60080/acp/alb-nginx:v3.12.2"
	e := NewEchoResty(base, cfg, log.L())
	err = e.Deploy(EchoCfg{Name: "echo-resty", Image: image, Ip: "v4"})
	assert.NoError(t, err)
}
