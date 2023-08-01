package test_utils

import (
	"testing"
)

func TestDockerHasImage(t *testing.T) {
	d := NewDockerExt(ConsoleLog())
	has, err := d.HasImage("registry.alauda.cn:60080/acp/alb2:local")
	t.Logf("%v %v", has, err)
}
