package test_utils

import (
	"context"

	cli "alauda.io/alb2/controller/cli"
	. "alauda.io/alb2/controller/types"
	drv "alauda.io/alb2/driver"
	"github.com/go-logr/logr"
)

type PolicyGetCtx struct {
	Ctx  context.Context
	Name string
	Ns   string
	Drv  *drv.KubernetesDriver
	L    logr.Logger
}

func GetPolicy(ctx PolicyGetCtx) (*NgxPolicy, error) {
	acli := cli.NewAlbCli(ctx.Drv, ctx.L)
	pcli := cli.NewPolicyCli(ctx.Drv, ctx.L, cli.PolicyCliOpt{MetricsPort: 0})
	lb, err := acli.GetLBConfig(ctx.Ns, ctx.Name)
	if err != nil {
		return nil, err
	}
	err = pcli.FillUpBackends(lb)
	if err != nil {
		return nil, err
	}
	policy := pcli.GenerateAlbPolicy(lb)
	return &policy, nil
}
