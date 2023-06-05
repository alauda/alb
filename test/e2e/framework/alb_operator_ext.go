package framework

import (
	"context"
	"fmt"
	"os"
	"path"
	"time"

	"alauda.io/alb2/pkg/operator/config"

	"alauda.io/alb2/pkg/operator/controllers"
	"alauda.io/alb2/pkg/operator/controllers/depl"
	cliu "alauda.io/alb2/utils/client"

	. "alauda.io/alb2/utils/test_utils"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
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

func (a *AlbOperatorExt) Start(ctx context.Context) {
	scheme := runtime.NewScheme()
	controllers.InitScheme(scheme)
	mgr, err := ctrl.NewManager(a.kubeCfg, ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: "0",
		LeaderElection:     false,
	})
	if err != nil {
		os.Exit(1)
	}
	cf := config.DEFAULT_OPERATOR_CFG
	if err = (&controllers.ALB2Reconciler{
		Client:     mgr.GetClient(),
		OperatorCf: cf,
		Log:        a.log,
	}).SetupWithManager(mgr); err != nil {
		a.log.Error(err, "unable to create controller", "controller", "ALB2")
		os.Exit(1)
	}

	if err := mgr.Start(ctx); err != nil {
		a.log.Error(err, "problem running manager")
		os.Exit(1)
	}

}

func (a *AlbOperatorExt) AssertDeployAlb(name types.NamespacedName, operatorEnv *config.OperatorCfg) {
	// use operator to create other resources.
	if operatorEnv == nil {
		operatorEnv = &config.DEFAULT_OPERATOR_CFG
	}
	cli := *a.client
	env := *operatorEnv
	log := a.log
	ctx := a.ctx
	count := 0
	for {
		count++
		log.Info("mock operator reconcile", "count", count)
		cur, err := depl.LoadAlbDeploy(ctx, cli, log, name, env)
		if err != nil {
			log.Error(err, "load err")
			continue
		}
		conf, err := config.NewALB2Config(cur.Alb, log)
		if err != nil {
			log.Error(err, "new err")
			continue
		}
		cfg := config.Config{Operator: env, ALB: *conf}
		dctl := depl.NewAlbDeployCtl(ctx, cli, cfg, log)
		expect, err := dctl.GenExpectAlbDeploy(ctx, cur)
		if err != nil {
			log.Error(err, "gen err")
			continue
		}
		redo, err := dctl.DoUpdate(ctx, cur, expect)
		if err != nil {
			log.Error(err, "update err")
		}
		if count > 100 {
			GinkgoNoErr(fmt.Errorf("too many times"))
		}
		if !redo && err == nil {
			time.Sleep(time.Millisecond * 100)
			break
		}
	}
	log.Info("mock operator reconcile", "count", count)
}

// 当我们想部署一个alb时，需要提供1. alb的配置，即cr，2. operator的配置,即operator的env
func (a *AlbOperatorExt) AssertDeploy(name types.NamespacedName, cfg string, operatorEnv *config.OperatorCfg) {
	a.log.Info("apply alb", "alb", cfg)
	a.kubectl.AssertKubectlApply(cfg)
	// use operator to create other resources.
	a.AssertDeployAlb(name, operatorEnv)
}
