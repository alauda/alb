package test_utils

import (
	"context"

	"alauda.io/alb2/config"
	cli "alauda.io/alb2/controller/cli"
	ct "alauda.io/alb2/controller/types"
	drv "alauda.io/alb2/driver"
	. "alauda.io/alb2/pkg/controller/ngxconf"
	. "alauda.io/alb2/pkg/controller/ngxconf/types"
	"github.com/go-logr/logr"
)

type PolicyGetCtx struct {
	Ctx  context.Context
	Name string
	Ns   string
	Drv  *drv.KubernetesDriver
	L    logr.Logger
	Cfg  *config.Config
}

func GetPolicy(ctx PolicyGetCtx) (*ct.NgxPolicy, error) {
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

func GetPolicyAndNgx(ctx PolicyGetCtx) (*ct.NgxPolicy, *NginxTemplateConfig, error) {
	acli := cli.NewAlbCli(ctx.Drv, ctx.L)
	pcli := cli.NewPolicyCli(ctx.Drv, ctx.L, cli.PolicyCliOpt{MetricsPort: 0})
	ncli := NewNgxCli(ctx.Drv, ctx.L, NgxCliOpt{})
	lb, err := acli.GetLBConfig(ctx.Ns, ctx.Name)
	if err != nil {
		return nil, nil, err
	}

	err = pcli.FillUpBackends(lb)
	if err != nil {
		return nil, nil, err
	}
	policy := pcli.GenerateAlbPolicy(lb)

	err = ncli.FillUpRefCms(lb)
	if err != nil {
		return nil, nil, err
	}
	tmpl, err := ncli.GenerateNginxTemplateConfig(lb, "running", ctx.Cfg)
	if err != nil {
		return nil, nil, err
	}
	return &policy, tmpl, nil
}
