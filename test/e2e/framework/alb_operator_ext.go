package framework

import (
	"context"
	"os"
	"path"

	"alauda.io/alb2/pkg/operator/config"
	"alauda.io/alb2/pkg/operator/controllers/depl"
	cliu "alauda.io/alb2/utils/client"
	. "alauda.io/alb2/utils/test_utils"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	ctlClient "sigs.k8s.io/controller-runtime/pkg/client"
)

// a wrapper of alb operator
// 这里并不想启动operator走reconcile的流程，直接复用operator部署的代码来创建相关的资源。这样比较简单
type AlbOperatorExt struct {
	base    string
	kubeCfg *rest.Config
	client  *ctlClient.Client
	kubectl *Kubectl
	ctx     context.Context
	log     logr.Logger
}

func NewAlbOperatorExt(ctx context.Context, base string, kubecfg *rest.Config) *AlbOperatorExt {
	base = path.Join(base, "operator")
	os.Mkdir(base, 0777)
	kubectl := NewKubectl(base, kubecfg, GinkgoLog())
	cli, err := cliu.GetClient(ctx, kubecfg, cliu.InitScheme(runtime.NewScheme()))
	GinkgoNoErr(err)
	return &AlbOperatorExt{ctx: ctx, base: base, kubeCfg: kubecfg, kubectl: kubectl, client: &cli, log: GinkgoLog()}
}

// 当我们想部署一个alb时，需要提供1. alb的配置，即cr，2. operator的配置,即operator的env
func (a *AlbOperatorExt) AssertDeploy(name types.NamespacedName, cfg string, operatorEnv *config.OperatorCfg) {
	a.log.Info("apply alb", "alb", cfg)
	a.kubectl.AssertKubectlApply(cfg)

	// use operator to create other resources.
	if operatorEnv == nil {
		operatorEnv = &config.DEFAULT_OPERATOR_CFG
	}
	cli := *a.client
	env := *operatorEnv
	log := a.log
	ctx := a.ctx
	cur, err := depl.LoadAlbDeploy(ctx, cli, log, name)
	GinkgoNoErr(err)
	conf, err := config.NewALB2Config(cur.Alb, log)
	GinkgoNoErr(err)
	dctl := depl.NewAlbDeployCtl(cli, env, log, conf)
	expect, err := dctl.GenExpectAlbDeploy(ctx, cur)
	GinkgoNoErr(err)
	_, err = dctl.DoUpdate(ctx, cur, expect)
	GinkgoNoErr(err)
}
