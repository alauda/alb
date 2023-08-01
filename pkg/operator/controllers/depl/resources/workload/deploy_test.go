package workload

import (
	"reflect"
	"testing"

	a2t "alauda.io/alb2/pkg/apis/alauda/v2beta1"
	. "alauda.io/alb2/pkg/config"
	"alauda.io/alb2/pkg/operator/config"
	. "alauda.io/alb2/utils/test_utils"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestEqual(t *testing.T) {
	a := map[string]map[string]string{
		"2": {
			"1": "1",
			"2": "2",
		},
		"1": {
			"1": "1",
			"2": "2",
		},
	}
	b := map[string]map[string]string{
		"1": {
			"2": "2",
			"1": "1",
		},
		"2": {
			"1": "1",
			"2": "2",
		},
	}
	assert.Equal(t, reflect.DeepEqual(a, b), true, "")
}

func TestTemplate(t *testing.T) {
	// ops := SetImage("nginx", "xx")
	name := "name"
	ns := "ns"
	alb := a2t.ALB2{
		ObjectMeta: v1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
	}

	cfg := &config.ALB2Config{
		ALBRunConfig: ALBRunConfig{
			Name: name,
			Ns:   ns,
		},
	}
	cfg.Overwrite = config.OverwriteCfg{
		Image: []a2t.ExternalImageOverwriteConfig{
			{
				Alb:   "alb.img",
				Nginx: "xx",
			},
		},
	}
	defaultc := config.DefaultConfig()
	defaultc.LoadbalancerName = &name
	err := cfg.Merge(defaultc)
	assert.NoError(t, err)
	dep := NewTemplate(&alb, nil, cfg, config.DEFAULT_OPERATOR_CFG, ConsoleLog()).Generate()
	assert.Equal(t, "xx", dep.Spec.Template.Spec.Containers[0].Image)
	assert.Equal(t, "alb.img", dep.Spec.Template.Spec.Containers[1].Image)
	t.Logf("depl %v", PrettyCr(dep))
}
