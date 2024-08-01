package cmd

import (
	"context"
	"os"
	"testing"

	"alauda.io/alb2/config"
	"alauda.io/alb2/driver"
	. "alauda.io/alb2/pkg/utils/test_utils"
	lu "alauda.io/alb2/utils"
	"alauda.io/alb2/utils/log"
	"github.com/stretchr/testify/assert"
)

func TestPolicyInReal(t *testing.T) {
	if os.Getenv("TestPolicyInReal") == "" {
		return
	}
	ctx := context.Background()
	mock := config.DefaultMock()
	l := log.L()
	kconf, err := driver.GetKubeCfgFromFile(os.Getenv("KUBECONFIG"))
	assert.NoError(t, err)
	drv, err := driver.NewDriver(driver.DrvOpt{Ctx: ctx, Cf: kconf, Opt: driver.Cfg2opt(mock)})
	assert.NoError(t, err)
	policy, err := GetPolicy(PolicyGetCtx{Ctx: ctx, Name: "global-alb2", Ns: "cpaas-system", Drv: drv, L: l})
	assert.NoError(t, err)
	_ = policy
	_ = lu.PrettyJson(policy)
	l.Info("p", "p", lu.PrettyJson(policy))
}
