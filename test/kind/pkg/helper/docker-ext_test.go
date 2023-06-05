package helper

import (
	"testing"

	"alauda.io/alb2/utils/log"
)

func TestDockerHasImage(t *testing.T) {
	d := NewDockerExt(log.L())
	has, err := d.HasImage("registry.alauda.cn:60080/acp/alb2:local")
	t.Logf("%v %v", has, err)
}
