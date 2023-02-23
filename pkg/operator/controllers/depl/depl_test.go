package depl

import (
	"fmt"
	"testing"

	"alauda.io/alb2/pkg/apis/alauda/v2beta1"
	"alauda.io/alb2/pkg/operator/toolkit"
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
	cfg := config.ALB2Config{
		Name: "test-alb",
		Deploy: config.DeployConfig{
			Replicas: 3,
		},
	}
	octl := NewAlbDeployCtl(nil, config.DEFAULT_OPERATOR_CFG, log, &cfg)
	depl, err := octl.genExpectDeployment(&AlbDeploy{Alb: alb}, &cfg)
	assert.NoError(t, err)
	log.Info("depl", "depl", fmt.Sprintf("%v", toolkit.PrettyJson(depl)))
	fmt.Printf("depl %s", toolkit.PrettyJson(depl))
}
