package framework

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"alauda.io/alb2/config"
	ctl "alauda.io/alb2/controller"
	albCtl "alauda.io/alb2/controller/alb"
	alb2v1 "alauda.io/alb2/pkg/apis/alauda/v1"
	"alauda.io/alb2/utils/log"
	. "alauda.io/alb2/utils/test_utils"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

type Env struct {
	*OperatorEnv
	albkubecfg *rest.Config
	bootYaml   string
	AlbNs      string
	AlbName    string
	*K8sClient
	*AlbWaitFileExt
	*ProductNs
	*IngressExt
	*SvcExt
	Opt       AlbEnvOpt
	ctlChan   chan AlbCtl
	albCtx    context.Context
	albCancel func()
}

type AlbEnvOpt struct {
	BootYaml string
	Ns       string
	Name     string
	StartAlb bool
	OperatorEnvCfg
}

func NewAlbEnvWithOpt(opt AlbEnvOpt) *Env {
	opt.RunOpext = true
	op := StartOperatorEnvOrDieWithOpt(opt.OperatorEnvCfg)
	a := &Env{
		OperatorEnv: op,
		bootYaml:    opt.BootYaml,
		AlbNs:       opt.Ns,
		AlbName:     opt.Name,
		K8sClient:   op.Kc,
		Opt:         opt,
	}
	a.ProductNs = &ProductNs{env: a}
	a.IngressExt = &IngressExt{
		kc:     op.Kc,
		ns:     opt.Ns,
		domain: op.Opext.operatorCfg.GetLabelBaseDomain(),
		ctx:    op.Ctx,
		log:    op.Log,
	}
	a.Kc.CreateNsIfNotExist(opt.Ns)
	a.SvcExt = NewSvcExt(a.Kc, a.Ctx)
	op.Kt.AssertKubectlApply(opt.BootYaml)

	// wait for alb deploy
	Wait(func() (bool, error) {
		_, err := a.GetDeploymentEnv(a.AlbNs, a.AlbName, "alb2")
		if err != nil {
			op.Log.Error(err, "get env fail")
			return false, nil
		}
		return true, nil
	})
	albkubecfg, err := a.GenAlbKubecfg(a.Ctx)
	if err != nil {
		GinkgoAssert(err, "gen alb kubecfg fail")
	}
	a.albkubecfg = albkubecfg
	if err := a.Start(); err != nil {
		GinkgoAssert(err, "start alb env fail")
	}
	return a
}

func NewAlbEnv(alb string, ns string, name string) *Env {
	opt := AlbEnvOpt{
		BootYaml: alb,
		Ns:       ns,
		Name:     name,
		StartAlb: true,
	}
	return NewAlbEnvWithOpt(opt)
}

type AlbCtl struct {
	kind string
}

type AlbInfo struct {
	AlbName      string
	AlbNs        string
	Domain       string
	NginxCfgPath string
	PolicyPath   string
}

func (a *Env) Start() error {
	l := a.Log
	base := a.Base
	root := os.Getenv("ALB_ROOT")
	kc := a.Kc
	ctx := a.Ctx

	l.Info("alb env ", "opt", a.Opt)
	nginxCfgPath := base + "/nginx.conf"
	nginxPolicyPath := base + "/policy.new"

	os.WriteFile(nginxCfgPath, []byte(""), os.ModePerm) // give it a default empty nginx.conf
	l.Info("apiserver", "host", a.cfg.Host, "token", a.cfg.BearerToken)

	tweakDir := base + "/tweak"
	os.MkdirAll(tweakDir, os.ModePerm)
	nginxTemplatePath, err := filepath.Abs(filepath.Join(root, "template/nginx/nginx.tmpl"))
	assert.Nil(GinkgoT(), err, "nginx template")
	assert.FileExists(GinkgoT(), nginxTemplatePath, "nginx template")

	statusDir := base + "/last_status"
	os.MkdirAll(statusDir, os.ModePerm)
	alb := a

	albWait := NewAlbWaitFileExt(&DefaultReadFile{}, AlbInfo{
		AlbName:      a.AlbName,
		AlbNs:        a.AlbNs,
		Domain:       a.Opext.operatorCfg.GetLabelBaseDomain(),
		NginxCfgPath: nginxCfgPath,
		PolicyPath:   nginxPolicyPath,
	}, l)
	l.Info("init wait file ext")
	a.AlbWaitFileExt = albWait

	openv, err := kc.GetDeploymentEnv(alb.AlbNs, alb.AlbName, "alb2")
	if err != nil {
		return err
	}
	for key, val := range openv {
		l.Info("env from depl ", "key", key, "val", val)
	}

	cfg := *config.InitFromEnv(openv)
	cfg.NginxTemplatePath = nginxTemplatePath
	cfg.NewConfigPath = nginxCfgPath + ".new"
	cfg.OldConfigPath = nginxCfgPath
	cfg.NewPolicyPath = nginxPolicyPath
	cfg.K8s.Mode = "kube_xx"
	cfg.K8s.K8sServer = a.albkubecfg.Host
	cfg.K8s.K8sToken = a.albkubecfg.BearerToken
	cfg.E2eTestControllerOnly = true
	cfg.TweakDir = tweakDir
	cfg.Pod = "p1"
	cfg.StatusFileParentPath = statusDir
	cfg.Leader = config.LeaderConfig{
		LeaseDuration: time.Second * time.Duration(3000),
		RenewDeadline: time.Second * time.Duration(2000),
		RetryPeriod:   time.Second * time.Duration(1000),
	}
	log.InTestSetLogger(a.Log)
	config.InTestSetConfig(cfg)
	ctlchan := make(chan AlbCtl, 10)
	a.ctlChan = ctlchan
	albctx, albcancel := context.WithCancel(ctx)
	a.albCtx = albctx
	a.albCancel = albcancel
	if a.Opt.StartAlb {
		go a.StartTestAlbLoop(a.albkubecfg, &cfg, l, ctlchan)
		if cfg.IsEnableAlb() {
			a.initDefaultFt()
			l.Info("wait alb normal")
			a.waitAlbNormal()
			l.Info("alb is normal")
		}
	}
	return nil
}

type AlbEnv = Env

func (a *AlbEnv) GenAlbKubecfg(ctx context.Context) (*rest.Config, error) {
	depl, err := a.K8sClient.GetK8sClient().AppsV1().Deployments(a.AlbNs).Get(ctx, a.AlbName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	sa := depl.Spec.Template.Spec.ServiceAccountName
	token, err := CreateToken(a.Kt, sa, a.AlbNs)
	if err != nil {
		return nil, err
	}
	return CreateRestCfg(a.cfg, token), nil
}

func (a *AlbEnv) StartTestAlbLoop(rest *rest.Config, cfg *config.Config, log logr.Logger, ctlchan chan AlbCtl) {
	// 本质上是一个alb的运行环境
	for {
		ctx := a.albCtx
		le := ctl.NewLeaderElection(ctx, cfg, rest, log)
		alb := albCtl.NewAlb(ctx, rest, cfg, le, log)
		alb.Start()
		ctl := <-ctlchan
		if ctl.kind == "stop" {
			break
		}
		if ctl.kind == "restart" {
			continue
		}
		panic(fmt.Sprintf("unknown event %v", ctl))
	}
}

func (f *AlbEnv) RestartAlb() {
	oldc := f.albCancel
	ctx, cancel := context.WithCancel(f.Ctx)
	f.albCtx = ctx
	f.albCancel = cancel

	f.ctlChan <- AlbCtl{
		kind: "restart",
	}
	oldc()
}

func (a *AlbEnv) waitAlbNormal() {
	a.WaitNginxConfigStr("listen.*12345")
	a.WaitPolicyRegex("12345")
}

func (a *AlbEnv) initDefaultFt() {
	kc := a.Kc
	ns := a.AlbNs
	name := a.AlbName
	domain := a.Opext.operatorCfg.GetLabelBaseDomain()
	ctx := a.Ctx
	defaultFt := 12345
	_, err := kc.GetAlbClient().CrdV1().Frontends(ns).Create(ctx, &alb2v1.Frontend{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      fmt.Sprintf("%s-%05d", name, defaultFt),
			// the most import part
			Labels: map[string]string{
				fmt.Sprintf("alb2.%s/name", domain): name,
			},
		},
		Spec: alb2v1.FrontendSpec{
			Port:     alb2v1.PortNumber(defaultFt),
			Protocol: alb2v1.FtProtocolHTTP,
		},
	}, metav1.CreateOptions{})

	GinkgoAssert(err, "init default ft fail")
}
