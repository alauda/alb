package test_utils

import (
	"context"
	"time"

	"alauda.io/alb2/config"
	cli "alauda.io/alb2/controller/cli"
	ct "alauda.io/alb2/controller/types"
	drv "alauda.io/alb2/driver"
	. "alauda.io/alb2/pkg/controller/ngxconf"
	. "alauda.io/alb2/pkg/controller/ngxconf/types"
	pm "alauda.io/alb2/pkg/utils/metrics"
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
	acli.CollectAndFetchRefs(lb)
	policy := pcli.GenerateAlbPolicy(lb)
	return &policy, nil
}

type XCli struct {
	alb    cli.AlbCli
	policy cli.PolicyCli
	nginx  NgxCli
}

func NewXCli(ctx PolicyGetCtx) XCli {
	s := time.Now()
	defer func() {
		pm.Write("test/init-policy-cli", float64(time.Since(s).Milliseconds()))
	}()
	acli := cli.NewAlbCli(ctx.Drv, ctx.L)
	pcli := cli.NewPolicyCli(ctx.Drv, ctx.L, cli.PolicyCliOpt{MetricsPort: 0})
	ncli := NewNgxCli(ctx.Drv, ctx.L, NgxCliOpt{})
	return XCli{
		alb:    acli,
		policy: pcli,
		nginx:  ncli,
	}
}

func (c XCli) GetPolicyAndNgx(ctx PolicyGetCtx) (*ct.NgxPolicy, *NginxTemplateConfig, error) {
	acli := c.alb
	pcli := c.policy
	ncli := c.nginx
	lb, err := acli.GetLBConfig(ctx.Ns, ctx.Name)
	if err != nil {
		return nil, nil, err
	}

	err = pcli.FillUpBackends(lb)
	if err != nil {
		return nil, nil, err
	}

	acli.CollectAndFetchRefs(lb)
	policy := pcli.GenerateAlbPolicy(lb)

	tmpl, err := ncli.GenerateNginxTemplateConfig(lb, "running", ctx.Cfg)
	if err != nil {
		return nil, nil, err
	}
	return &policy, tmpl, nil
}

func GetPolicyAndNgx(ctx PolicyGetCtx) (*ct.NgxPolicy, *NginxTemplateConfig, error) {
	return NewXCli(ctx).GetPolicyAndNgx(ctx)
}
