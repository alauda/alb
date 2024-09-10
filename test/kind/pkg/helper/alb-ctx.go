package helper

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/ztrue/tracerr"
	"k8s.io/client-go/rest"

	"alauda.io/alb2/utils/log"
	. "alauda.io/alb2/utils/test_utils"
	. "alauda.io/alb2/utils/test_utils/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type KindConfigExt struct {
	kindcfg      KindConfig
	workerlabels []string
	stack        string
	extraCrds    []string
}

type AlbK8sCfg struct {
	Base           string
	chart          string
	kindcfg        KindConfigExt
	Alboperatorcfg AlbOperatorCfg
	lbsvccfg       LbsvcCtlCfg
}

func NewAlbK8sCfg() *AlbK8sCfg {
	return &AlbK8sCfg{}
}

func (c *AlbK8sCfg) WithChart(chart string) *AlbK8sCfg {
	c.chart = chart
	return c
}

func (c *AlbK8sCfg) WithDefaultAlbName(name string) *AlbK8sCfg {
	c.Alboperatorcfg.DefaultAlbName = name
	return c
}

func (c *AlbK8sCfg) UseMockLBSvcCtl(v4p, v6p []string) *AlbK8sCfg {
	c.lbsvccfg.mockmetallb = true
	c.lbsvccfg.v4pool = v4p
	c.lbsvccfg.v6pool = v6p
	return c
}

func (c *AlbK8sCfg) UseMetalLBSvcCtl(v4p, v6p []string) *AlbK8sCfg {
	c.lbsvccfg.metallb = true
	c.lbsvccfg.v4pool = v4p
	c.lbsvccfg.v6pool = v6p
	return c
}

func (c *AlbK8sCfg) WithDefaultOverWrite(cfg string) *AlbK8sCfg {
	c.Alboperatorcfg.chartCfgs = []string{cfg}
	return c
}

func (c *AlbK8sCfg) WithWorkerLabels(lable []string) *AlbK8sCfg {
	c.kindcfg.workerlabels = lable
	return c
}

func (c *AlbK8sCfg) WithKindNode(image string) *AlbK8sCfg {
	c.kindcfg.kindcfg.Image = image
	return c
}

func (c *AlbK8sCfg) DisableDefaultAlb() *AlbK8sCfg {
	c.Alboperatorcfg.DisableDefaultAlb = true
	return c
}

func (c *AlbK8sCfg) Build() AlbK8sCfg {
	base := InitBase()
	c.Base = base
	kindName := "ldev"
	if c.chart == "" {
		chartFromenv := os.Getenv("ALB_KIND_E2E_CHART")
		if chartFromenv != "" {
			// CI 环境
			c.chart = fmt.Sprintf("registry.alauda.cn:60080/acp/chart-alauda-alb2:%s", chartFromenv)
		}
	}

	if os.Getenv("DEV_MODE") != "true" {
		kindName = fmt.Sprintf("%s-%d", kindName, time.Now().Unix())
	}
	if c.kindcfg.kindcfg.Name == "" {
		c.kindcfg.kindcfg.Name = kindName
	}
	if c.kindcfg.stack == "" {
		c.kindcfg.stack = "ipv4"
	}
	if c.kindcfg.kindcfg.Image == "" {
		c.kindcfg.kindcfg.Image = "kindest/node:v1.24.3"
	}
	c.kindcfg = KindConfigExt{
		kindcfg: KindConfig{
			Base:  c.Base,
			Name:  kindName,
			Image: c.kindcfg.kindcfg.Image,
			ClusterYaml: Template(`
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
networking:
    ipFamily: {{.ipFamily}}
    apiServerAddress: "127.0.0.1"
nodes:
   - role: control-plane
   {{- range $w := .workers }}
   {{ $w }}
   {{- end }}
containerdConfigPatches:
- |-
    [plugins."io.containerd.grpc.v1.cri".registry.mirrors."registry.alauda.cn:60080"] endpoint = ["http://registry.alauda.cn:60080"]
`, map[string]interface{}{"ipFamily": c.kindcfg.stack, "workers": c.kindcfg.workerlabels}),
		},
	}
	return *c
}

type AlbK8sCtx struct {
	Cfg      AlbK8sCfg
	Ctx      context.Context
	Kind     *KindExt
	Kubecfg  *rest.Config
	opctl    *AlbOperatorCtl
	albChart *AlbChartExt
	Log      logr.Logger
}

func NewAlbK8sCtx(ctx context.Context, cfg AlbK8sCfg) *AlbK8sCtx {
	return &AlbK8sCtx{Ctx: ctx, Cfg: cfg}
}

func (a *AlbK8sCtx) Init() error {
	l := log.L()
	l.Info("init", "cfg", a.Cfg)
	a.Log = l

	if err := a.initKind(); err != nil {
		return err
	}

	if err := a.initChart(); err != nil {
		return err
	}

	if err := a.initAlbOperator(); err != nil {
		return err
	}

	if err := a.initMetallb(); err != nil {
		return err
	}

	return nil
}

type LbsvcCtlCfg struct {
	mockmetallb bool
	metallb     bool
	v4pool      []string
	v6pool      []string
}

func (a *AlbK8sCtx) initMetallb() error {
	cfg := a.Cfg.lbsvccfg
	if cfg.mockmetallb {
		m := NewMockMetallb(a.Ctx, a.Kubecfg, cfg.v4pool, cfg.v6pool, []string{}, a.Log)
		go m.Start()
	}
	if cfg.metallb {
		m := NewMetallbInstaller(a.Ctx, a.Cfg.Base, a.Kubecfg, a.Kind, cfg.v4pool, cfg.v6pool, a.Log)
		m.Init()
	}
	return nil
}

func (a *AlbK8sCtx) initAlbOperator() error {
	l := a.Log
	opctl, err := NewAlbOperatorCtl(a.Ctx, a.Cfg.Base, a.Log, a.Kind, a.albChart)
	if err != nil {
		return err
	}
	a.opctl = opctl
	l.Info("clean alb operator")
	opctl.clean()
	l.Info("deploy alb operator chart")
	if err := opctl.init(a.Cfg.Alboperatorcfg); err != nil {
		return err
	}

	return nil
}

func (a *AlbK8sCtx) DeployEchoResty() error {
	e := NewEchoResty(a.Cfg.Base, a.Kubecfg, a.Log)
	_, nginx, err := a.albChart.GetImage()
	if err != nil {
		return err
	}
	a.Log.Info("nginx", "image", nginx)
	return e.Deploy(EchoCfg{Name: "echo-resty", Image: nginx, Ip: "v4"})
}

func (a *AlbK8sCtx) Destroy() error {
	if os.Getenv("DEV_MODE") != "true" {
		KindDelete(a.Cfg.kindcfg.kindcfg.Name)
	}
	return nil
}

func (a *AlbK8sCtx) initKind() error {
	l := a.Log
	kind, err := DeployOrAdopt(a.Cfg.kindcfg.kindcfg, a.Cfg.Base, a.Cfg.kindcfg.kindcfg.Name, l)
	if err != nil {
		return err
	}
	cfg, err := kind.GetConfig()
	a.Kubecfg = cfg
	if err != nil {
		return err
	}
	a.Kind = kind
	kc := NewKubectl(a.Cfg.Base, cfg, l)
	l.Info("kind", "name", kind.Name)
	for i, label := range a.Cfg.kindcfg.workerlabels {
		i++
		name := fmt.Sprintf("%d", i)
		if i == 1 {
			name = ""
		}
		kc.Kubectl(fmt.Sprintf("label nodes %s-worker%s %s", kind.Name, name, label))
	}

	l.Info("install extra crs ", "cr", a.Cfg.kindcfg.extraCrds)
	for _, p := range a.Cfg.kindcfg.extraCrds {
		_, err = kc.Kubectl("apply", "-R", "-f", p, "--force")
		if err != nil {
			return err
		}
	}
	return nil
}

func (a *AlbK8sCtx) initChart() error {
	albchart, err := NewAlbChart().WithBase(a.Cfg.Base).WithLog(a.Log).Load(a.Cfg.chart)
	if err != nil {
		return err
	}
	a.albChart = albchart
	return nil
}

type AlbOperatorCtl struct {
	log     logr.Logger
	kind    *KindExt
	chart   *AlbChartExt
	docker  *DockerExt
	Kubectl *Kubectl
	Kubecli *K8sClient
	helm    *Helm
	cfg     AlbOperatorCfg
	ctx     context.Context
}

func NewAlbOperatorCtl(ctx context.Context, base string, log logr.Logger, kind *KindExt, chart *AlbChartExt) (*AlbOperatorCtl, error) {
	l := log
	d := NewDockerExt(log.WithName("docker"))
	cfg, err := kind.GetConfig()
	if err != nil {
		return nil, err
	}
	helm := NewHelm(base, cfg, l)
	Kubectl := NewKubectl(base, cfg, l)
	cli := NewK8sClient(ctx, cfg)
	return &AlbOperatorCtl{ctx: ctx, log: l, kind: kind, chart: chart, docker: &d, helm: helm, Kubectl: Kubectl, Kubecli: cli}, nil
}

type AlbOperatorCfg struct {
	chartCfgs         []string
	OperatorNs        string
	DefaultAlbName    string
	DefaultAlbNS      string
	DisableDefaultAlb bool
}

func (a *AlbOperatorCtl) init(cfg AlbOperatorCfg) error {
	a.cfg = cfg
	l := a.log
	docker := a.docker
	helm := a.helm
	if cfg.OperatorNs == "" {
		cfg.OperatorNs = "kube-system"
	}
	if cfg.DefaultAlbName == "" {
		cfg.DefaultAlbName = ""
	}
	if cfg.DefaultAlbNS == "" {
		cfg.DefaultAlbNS = "default"
	}
	// load image in kind
	images, err := a.chart.ListImage()
	if err != nil {
		return err
	}
	err = docker.PullIfNotFound(images...)
	if err != nil {
		return err
	}
	l.Info("load images into kind", "images", images)
	err = a.kind.LoadImage(images...)
	if err != nil {
		l.Error(err, "load image failed", "images", images)
		return tracerr.Wrap(err)
	}

	chart := a.chart.GetChartDir()
	out, err := helm.Install(cfg.chartCfgs, "alb-operator", chart, chart+"/values.yaml")
	if err != nil {
		return err
	}
	l.Info("chart install", "out", out)

	assert := NewDeploymentAssert(a.Kubecli, l)
	assert.WaitReady(a.ctx, "alb-operator-ctl", cfg.OperatorNs)
	if !cfg.DisableDefaultAlb {
		assert.WaitReady(a.ctx, cfg.DefaultAlbName, cfg.DefaultAlbNS)
	}
	return nil
}

func (a *AlbOperatorCtl) clean() error {
	rs, err := a.Kubectl.Kubectl("api-resources")
	l := a.log
	ns := a.cfg.DefaultAlbNS
	kcli := a.Kubecli
	if err != nil {
		return err
	}
	if strings.Contains(rs, "alaudaloadbalancer2") {
		// delete all alb
		l.Info("delete all albs")
		Wait(func() (bool, error) {
			{
				albs, err := kcli.GetAlbClient().CrdV2beta1().ALB2s(ns).List(a.ctx, metav1.ListOptions{})
				if err != nil {
					return false, err
				}
				for _, alb := range albs.Items {
					alb.Finalizers = nil
					kcli.GetAlbClient().CrdV2beta1().ALB2s(ns).Update(a.ctx, &alb, metav1.UpdateOptions{})
					kcli.GetAlbClient().CrdV2beta1().ALB2s(ns).Delete(a.ctx, alb.Name, metav1.DeleteOptions{})
				}
			}
			albs, err := kcli.GetAlbClient().CrdV2beta1().ALB2s(ns).List(a.ctx, metav1.ListOptions{})
			if err != nil {
				return false, err
			}
			l.Info("albs", "items", albs.Items)
			if len(albs.Items) != 0 {
				return false, nil
			}
			return true, nil
		})
	}
	// uninstall operator
	l.Info("uninstall helm")
	a.helm.UnInstallAll()
	l.Info("clean rbac")
	a.Kubectl.Kubectl("delete clusterrolebindings.rbac.authorization.k8s.io alb-operator")
	a.Kubectl.Kubectl("delete clusterroles.rbac.authorization.k8s.io alb-operator")
	return nil
}
