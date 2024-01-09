package framework

import (
	"context"
	"fmt"
	"os"
	"path"
	"time"

	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"alauda.io/alb2/pkg/operator/config"

	"alauda.io/alb2/pkg/operator/controllers"
	"alauda.io/alb2/pkg/operator/controllers/depl"
	cliu "alauda.io/alb2/utils/client"

	. "alauda.io/alb2/utils/test_utils"
	"github.com/go-logr/logr"
	"github.com/lithammer/dedent"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	ctlClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

// a wrapper of alb operator
type AlbOperatorExt struct {
	base            string
	adminKubeCfg    *rest.Config
	operatorKubeCfg *rest.Config
	adminClient     ctlClient.Client
	kubectl         *Kubectl
	ctx             context.Context
	operatorCfg     *config.OperatorCfg
	operatorClient  ctlClient.Client
	log             logr.Logger
}

func NewAlbOperatorExt(ctx context.Context, base string, kubecfg *rest.Config, log logr.Logger) *AlbOperatorExt {
	base = path.Join(base, "operator")
	os.Mkdir(base, 0o777)
	kubectl := NewKubectl(base, kubecfg, log)
	cli, err := cliu.GetClient(ctx, kubecfg, cliu.InitScheme(runtime.NewScheme()))
	GinkgoNoErr(err)
	return &AlbOperatorExt{
		ctx:          ctx,
		base:         base,
		operatorCfg:  &config.DEFAULT_OPERATOR_CFG,
		adminKubeCfg: kubecfg,
		kubectl:      kubectl,
		adminClient:  cli,
		log:          log,
	}
}

// use helm to install the operator chart and get kubecfg from service account
func (a *AlbOperatorExt) Init(ctx context.Context, defaultVal string, csvMode bool) error {
	var (
		albBase          = os.Getenv("ALB_ROOT")
		chartBase        = path.Join(albBase, "/deploy/chart/alb")
		defaultValuePath = path.Join(chartBase, "values.yaml")
	)
	helm := NewHelm(a.base, a.adminKubeCfg, a.log)
	defaultVal = dedent.Dedent(defaultVal)
	if defaultVal == "" {
		defaultVal = dedent.Dedent(`
            operatorDeployMode: "deployment"
            defaultAlb: false
            global:
              labelBaseDomain: cpaas.io
              namespace: cpaas-system
              registry:
                address: build-harbor.alauda.cn
		`)
	}
	helm.AssertInstall([]string{defaultVal}, "alb-operator", chartBase, defaultValuePath)
	// csv 模式下,安装chart之后创建csv,但是我们的envtest里是没有olm的..所有不会部署deployment
	if !csvMode {
		a.kubectl.VerboseKubectl("get sa -A")
		a.kubectl.VerboseKubectl("get deployment -A")
		cfg, err := a.genKubeCfgFromDeploymentServiceAccount(ctx, "alb-operator", "cpaas-system")
		if err != nil {
			return err
		}
		a.operatorKubeCfg = cfg
		cli, err := cliu.GetClient(ctx, cfg, cliu.InitScheme(runtime.NewScheme()))
		if err != nil {
			return err
		}
		a.operatorClient = cli
		sal := &corev1.ServiceAccountList{}
		err = cli.List(ctx, sal, &ctlClient.ListOptions{Namespace: "cpaas-system"})
		if err != nil {
			a.log.Error(err, "list service account failed")
			return err
		}
	}
	return nil
}

// we need to create token manually
func (a *AlbOperatorExt) genKubeCfgFromDeploymentServiceAccount(ctx context.Context, name string, ns string) (*rest.Config, error) {
	// we need to create token manually
	depl := appv1.Deployment{}
	err := a.adminClient.Get(ctx, types.NamespacedName{Name: name, Namespace: ns}, &depl)
	if err != nil {
		return nil, err
	}
	account := depl.Spec.Template.Spec.ServiceAccountName
	a.log.Info("account", "name", account)
	token, err := CreateToken(a.kubectl, name, ns)
	if err != nil {
		return nil, err
	}
	operatorKubecfg := &rest.Config{
		Host:        a.adminKubeCfg.Host,
		BearerToken: token,
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: true,
		},
	}
	return operatorKubecfg, nil
}

func (a *AlbOperatorExt) Start(ctx context.Context) {
	scheme := runtime.NewScheme()
	controllers.InitScheme(scheme)
	mgr, err := ctrl.NewManager(a.operatorKubeCfg, ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: "0",
		},
		LeaderElection: false,
	})
	if err != nil {
		os.Exit(1)
	}

	if err = controllers.Setup(mgr, *a.operatorCfg, a.log); err != nil {
		a.log.Error(err, "unable to create controller", "controller", "ALB2")
		os.Exit(1)
	}

	if err := mgr.Add(manager.RunnableFunc(func(ctx context.Context) error {
		return controllers.StandAloneGatewayClassInit(ctx, *a.operatorCfg, mgr.GetClient(), a.log)
	})); err != nil {
		a.log.Error(err, "add init runnable fail")
		os.Exit(1)
	}

	if err := mgr.Start(ctx); err != nil {
		a.log.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func (a *AlbOperatorExt) DeployAlb(name types.NamespacedName, operatorEnv *config.OperatorCfg) (bool, error) {
	if operatorEnv == nil {
		operatorEnv = &config.DEFAULT_OPERATOR_CFG
	}
	cli := a.operatorClient
	env := *operatorEnv
	log := a.log
	ctx := a.ctx
	cur, err := depl.LoadAlbDeploy(ctx, cli, log, name, env)
	if err != nil {
		log.Error(err, "load err")
		return false, err
	}
	conf, err := config.NewALB2Config(cur.Alb, *operatorEnv, log)
	if err != nil {
		log.Error(err, "new err")
		return false, err
	}
	cfg := config.Config{Operator: env, ALB: *conf}
	dctl := depl.NewAlbDeployCtl(ctx, cli, cfg, log)
	expect, err := dctl.GenExpectAlbDeploy(ctx, cur)
	if err != nil {
		log.Error(err, "gen err")
		return false, err
	}
	redo, reason, err := dctl.DoUpdate(ctx, cur, expect)
	if err != nil {
		log.Error(err, "update err")
		return false, err
	}
	if redo {
		log.Info("redo", "reason", reason)
	}
	return redo, nil
}

func (a *AlbOperatorExt) AssertDeployAlb(name types.NamespacedName, operatorEnv *config.OperatorCfg) {
	// use operator to create other resources.
	if operatorEnv == nil {
		operatorEnv = &config.DEFAULT_OPERATOR_CFG
	}
	log := a.log
	count := 0
	for {
		count++
		log.Info("mock operator reconcile", "count", count)
		if count > 100 {
			GinkgoNoErr(fmt.Errorf("too many times"))
		}
		redo, err := a.DeployAlb(name, operatorEnv)
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
