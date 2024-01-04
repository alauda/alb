package depl

import (
	"context"
	"fmt"
	"testing"

	. "alauda.io/alb2/pkg/config"

	"alauda.io/alb2/pkg/apis/alauda/v2beta1"
	"alauda.io/alb2/utils"
	"alauda.io/alb2/utils/test_utils"
	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"alauda.io/alb2/pkg/operator/config"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGenDeployment(t *testing.T) {
	log := zap.New()
	log.Info("in test")

	alb := &v2beta1.ALB2{
		ObjectMeta: metaV1.ObjectMeta{
			Name: "test-alb",
			UID:  "111",
		},
		Spec:   v2beta1.ALB2Spec{},
		Status: v2beta1.ALB2Status{},
	}
	cfg := config.Config{
		ALB: config.ALB2Config{
			ALBRunConfig: ALBRunConfig{
				Name: "test-alb",
			},
			Deploy: config.DeployConfig{
				Replicas: 3,
			},
		},
		Operator: config.DEFAULT_OPERATOR_CFG,
	}
	octl := NewAlbDeployCtl(context.Background(), nil, cfg, log)
	depl, err := octl.genExpectDeployment(&AlbDeploy{Alb: alb}, &cfg.ALB)
	assert.NoError(t, err)
	log.Info("depl", "depl", fmt.Sprintf("%v", utils.PrettyJson(depl)))
	fmt.Printf("depl %s", utils.PrettyJson(depl))
}

func TestProject(t *testing.T) {
	log := test_utils.ConsoleLog()
	log.Info("in test")

	alb := &v2beta1.ALB2{
		ObjectMeta: metaV1.ObjectMeta{
			Name: "test-alb",
			UID:  "111",
			Labels: map[string]string{
				"cpaas.io/role": "port",
				"a":             "b",
			},
		},
		Spec:   v2beta1.ALB2Spec{},
		Status: v2beta1.ALB2Status{},
	}
	cfg := config.Config{
		ALB: config.ALB2Config{
			ALBRunConfig: ALBRunConfig{
				Name: "test-alb",
			},
			Project: config.ProjectConfig{
				EnablePortProject: false,
			},
			Deploy: config.DeployConfig{
				Replicas: 3,
			},
		},
		Operator: config.DEFAULT_OPERATOR_CFG,
	}
	octl := NewAlbDeployCtl(context.Background(), nil, cfg, log)
	alb, err := octl.genExpectAlb(&AlbDeploy{Alb: alb}, &cfg.ALB)
	assert.NoError(t, err)
	assert.Equal(t, alb.Labels, map[string]string{"a": "b"})
	log.Info("depl", "depl", fmt.Sprintf("%v", utils.PrettyJson(alb.Labels)))
}
