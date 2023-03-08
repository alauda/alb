package workload

import (
	"testing"

	"alauda.io/alb2/pkg/operator/config"
	"github.com/stretchr/testify/assert"
)

func TestTemplate(t *testing.T) {
	ops := SetImage("nginx", "xx")
	dep := NewTemplate("ns", "name", "1", nil, &config.ALB2Config{}, config.DEFAULT_OPERATOR_CFG).Generate(ops)
	assert.Equal(t, "xx", dep.Spec.Template.Spec.Containers[0].Image)
	assert.Equal(t, "alb.img", dep.Spec.Template.Spec.Containers[1].Image)
	t.Logf("depl %v", dep)
}
