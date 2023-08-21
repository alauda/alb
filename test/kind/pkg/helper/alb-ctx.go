package helper

import (
	"context"
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/ztrue/tracerr"
	"k8s.io/client-go/rest"
	"k8s.io/utils/env"

	"alauda.io/alb2/pkg/apis/alauda/v2beta1"
	"alauda.io/alb2/utils/log"
	. "alauda.io/alb2/utils/test_utils"
	. "alauda.io/alb2/utils/test_utils/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TODO 将部署k8s和部署alb分开
type AlbK8sCfg struct {
	kindcfg             KindConfig
	base                string
	chart               string
	chartoverwrite      []string
	KindName            string
	DefaultAlbName      string
	Ns                  string
	version             string
	nginx               string
	extraCrs            []string
	stack               string
	mockmetallb         bool
	metallb             bool
	v4pool              []string
	v6pool              []string
	disableDefault      bool
	defaultAlbOverwrite string
	workerlabels        []string
}

func NewAlbK8sCfg() *AlbK8sCfg {
	return &AlbK8sCfg{}
}

func (c *AlbK8sCfg) WithChart(chart string) *AlbK8sCfg {
	c.chart = chart
	return c
}
func (c *AlbK8sCfg) WithKindName(name string) *AlbK8sCfg {
	c.KindName = name
	return c
}

func (c *AlbK8sCfg) WithDefaultAlbName(name string) *AlbK8sCfg {
	c.DefaultAlbName = name
	return c
}

func (c *AlbK8sCfg) UseMockLBSvcCtl(v4p, v6p []string) *AlbK8sCfg {
	c.mockmetallb = true
	c.v4pool = v4p
	c.v6pool = v6p
	return c
}

func (c *AlbK8sCfg) UseMetalLBSvcCtl(v4p, v6p []string) *AlbK8sCfg {
	c.metallb = true
	c.v4pool = v4p
	c.v6pool = v6p
	return c
}

func (c *AlbK8sCfg) DualStack() *AlbK8sCfg {
	c.stack = "dual"
	return c
}

func (c *AlbK8sCfg) WithDefaultOverWrite(cfg string) *AlbK8sCfg {
	c.defaultAlbOverwrite = cfg
	return c
}

func (c *AlbK8sCfg) WithWorkerLabels(lable []string) *AlbK8sCfg {
	c.workerlabels = lable
	return c
}

func (c *AlbK8sCfg) Ipv6() *AlbK8sCfg {
	c.stack = "ipv6"
	return c
}

func (c *AlbK8sCfg) DisableDefaultAlb() *AlbK8sCfg {
	c.disableDefault = true
	return c
}

func (c *AlbK8sCfg) Build() AlbK8sCfg {
	base := InitBase()
	c.base = base
	if c.KindName == "" {
		c.KindName = "ldev"
	}
	if c.chart == "" {
		chartFromenv := os.Getenv("ALB_KIND_E2E_CHART")
		if chartFromenv != "" {
			// CI 环境
			c.chart = fmt.Sprintf("registry.alauda.cn:60080/acp/chart-alauda-alb2:%s", chartFromenv)
		} else {
			c.chart = fmt.Sprintf("%s/deploy/chart/alb", os.Getenv("ALB_ROOT"))
		}
	}

	// add branch prefix
	kindName := fmt.Sprintf("%s-%d", c.KindName, time.Now().Unix())
	if os.Getenv("DEV_MODE") == "true" {
		kindName = c.KindName
	}

	workers := ""
	for _, _ = range c.workerlabels {
		workers = fmt.Sprintf("%s  - role: worker\n", workers)
	}

	c.kindcfg = KindConfig{
		Base:  c.base,
		Name:  kindName,
		Image: "kindest/node:v1.24.3",
		ClusterYaml: Template(`
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
networking:
  ipFamily: {{.ipFamily}}
  apiServerAddress: "127.0.0.1"
nodes:
  - role: control-plane
{{.workers}}
containerdConfigPatches:
- |-
   [plugins."io.containerd.grpc.v1.cri".registry.mirrors."registry.alauda.cn:60080"] endpoint = ["http://registry.alauda.cn:60080"]
`, map[string]interface{}{"ipFamily": c.stack, "workers": workers}),
	}
	if c.DefaultAlbName == "" {
		c.DefaultAlbName = "default-alb"
	}
	c.chartoverwrite = []string{
		Template(`
operator:
  albImagePullPolicy: IfNotPresent
operatorDeployMode: "deployment"
defaultAlb: {{.defaultAlb}}
operatorReplicas: 1
projects: [ALL_ALL]
displayName: ""
global:
    labelBaseDomain: cpaas.io
    namespace: cpaas-system
    registry:
      address: registry.alauda.cn:60080
loadbalancerName: {{.name}}
replicas: 1
`, map[string]interface{}{"name": c.DefaultAlbName, "defaultAlb": !c.disableDefault}),
		c.defaultAlbOverwrite,
	}
	ns := "cpaas-system"
	c.Ns = ns
	root := os.Getenv("ALB_ROOT")
	if root != "" {
		albroot := env.GetString("ALB_ROOT", "")
		c.extraCrs = append(c.extraCrs, path.Join(albroot, "scripts", "yaml", "crds", "extra", "v1"))
	}
	crs := os.Getenv("ALB_EXTRA_CRS")
	if crs != "" {
		c.extraCrs = append(c.extraCrs, crs)
	}
	c.nginx = "registry.alauda.cn:60080/acp/alb-nginx:v3.12.2"
	return *c
}

type AlbK8sExt struct {
	Kind         *KindExt
	docker       *DockerExt
	helm         *Helm
	Kubectl      *Kubectl
	Kubecliet    *K8sClient
	albChart     *AlbChartExt
	kubecfg      *rest.Config
	Log          logr.Logger
	deployAssert *DeploymentAssert
}

type AlbK8sCtx struct {
	Ctx context.Context
	Cfg AlbK8sCfg
	AlbK8sExt
}

func NewAlbK8sCtx(ctx context.Context, cfg AlbK8sCfg) *AlbK8sCtx {
	return &AlbK8sCtx{Ctx: ctx, Cfg: cfg}
}

func (a *AlbK8sCtx) initExt() error {
	l := a.Log
	ctx := a.Ctx
	base := a.Cfg.base
	kubecfg, err := a.Kind.GetConfig()
	a.kubecfg = kubecfg
	if err != nil {
		return err
	}

	d := NewDockerExt(l.WithName("docker"))
	a.docker = &d

	kc := NewK8sClient(ctx, kubecfg)
	a.Kubecliet = kc

	k := NewKubectl(base, kubecfg, l)
	a.Kubectl = k

	helm := NewHelm(base, kubecfg, l)
	a.helm = helm

	albchart, err := NewAlbChart().WithBase(base).WithHelm(helm).WithLog(l).Load(a.Cfg.chart)
	if err != nil {
		return err
	}
	a.albChart = albchart

	deployAssert := NewDeploymentAssert(kc, l)
	a.deployAssert = deployAssert

	return nil
}

func (a *AlbK8sCtx) Init() error {
	l := log.L()
	KindGC(l)
	l.Info("init", "cfg", a.Cfg)
	a.Log = l
	base := a.Cfg.base

	kind, err := DeployOrAdopt(a.Cfg.kindcfg, base, a.Cfg.kindcfg.Name, l)
	if err != nil {
		return err
	}
	a.Kind = kind
	l.Info("kind", "name", kind.Name)
	a.initExt()
	for i, label := range a.Cfg.workerlabels {
		i = i + 1
		name := fmt.Sprintf("%d", i)
		if i == 1 {
			name = ""
		}
		a.Kubectl.Kubectl(fmt.Sprintf("label nodes %s-worker%s %s", kind.Name, name, label))
	}
	version, err := a.albChart.GetVersion()
	if err != nil {
		return err
	}
	a.Cfg.version = version
	// check all nodes ready
	out, err := a.Kubectl.Kubectl("get", "nodes")
	if err != nil {
		return err
	}
	l.Info("x nodes", "out", out)
	{
		// load basic image used for test
		err := kind.LoadImage(a.Cfg.nginx)
		if err != nil {
			return tracerr.Wrap(err)
		}
	}
	l.Info("x nodes", "out", out)

	if err := a.DeployAlbOperator(); err != nil {
		return err
	}
	if a.Cfg.mockmetallb {
		m := NewMockMetallb(a.Ctx, a.kubecfg, a.Cfg.v4pool, a.Cfg.v6pool, []string{}, a.Log)
		go m.Start()
	}
	if a.Cfg.metallb {
		m := NewMetallbInstaller(a.Ctx, base, a.kubecfg, a.Kind, a.docker, a.Cfg.v4pool, a.Cfg.v6pool, a.Log)
		m.Init()
	}

	return nil
}

// 给定kubecfg部署一个alb
// 包括同步镜像之类的
func (a *AlbK8sCtx) DeployAlbOperator() error {
	l := a.Log
	err := a.CleanUpAll()
	if err != nil {
		return err
	}
	l.Info("deploy alb operator chart")
	if err := a.DeployOperator(); err != nil {
		return err
	}

	return nil
}

func (a *AlbK8sCtx) CleanUpAll() error {
	a.Log.Info("clean up first")
	rs, err := a.Kubectl.Kubectl("api-resources")
	if err != nil {
		return err
	}
	if strings.Contains(rs, "alaudaloadbalancer2") {
		// delete all alb
		a.Log.Info("delete all albs")
		Wait(func() (bool, error) {
			{
				albs, err := a.Kubecliet.GetAlbClient().CrdV2beta1().ALB2s(a.Cfg.Ns).List(a.Ctx, metav1.ListOptions{})
				if err != nil {
					return false, err
				}
				for _, alb := range albs.Items {
					alb.Finalizers = nil
					a.Kubecliet.GetAlbClient().CrdV2beta1().ALB2s(a.Cfg.Ns).Update(a.Ctx, &alb, metav1.UpdateOptions{})
					a.Kubecliet.GetAlbClient().CrdV2beta1().ALB2s(a.Cfg.Ns).Delete(a.Ctx, alb.Name, metav1.DeleteOptions{})
				}
			}
			albs, err := a.Kubecliet.GetAlbClient().CrdV2beta1().ALB2s(a.Cfg.Ns).List(a.Ctx, metav1.ListOptions{})
			if err != nil {
				return false, err
			}
			a.Log.Info("albs", "items", albs.Items)
			if len(albs.Items) != 0 {
				return false, nil
			}
			return true, nil
		})
	}
	// uninstall operator
	a.Log.Info("uninstall helm")
	a.helm.UnInstallAll()
	a.Log.Info("clean rbac")
	a.Kubectl.Kubectl("delete clusterrolebindings.rbac.authorization.k8s.io alb-operator")
	a.Kubectl.Kubectl("delete clusterroles.rbac.authorization.k8s.io alb-operator")
	return nil
}

func (a *AlbK8sCtx) DeployOperator() error {
	l := a.Log
	helm := a.helm
	ac := a.albChart
	d := a.docker
	kind := a.Kind
	kc := a.Kubecliet
	ns := a.Cfg.Ns
	kubectl := a.Kubectl
	name := a.Cfg.KindName
	cfgs := a.Cfg.chartoverwrite
	chartBase := ac.chart
	images, err := ac.ListImage()
	if err != nil {
		return err
	}
	err = d.PullIfNotFound(images...)
	if err != nil {
		return err
	}
	err = kind.LoadImage(images...)
	if err != nil {
		l.Error(err, "load image failed", "images", images)
		return tracerr.Wrap(err)
	}

	l.Info("create ns if not exist", "ns", ns)
	err = kc.CreateNsIfNotExist(ns)
	if err != nil {
		return err
	}
	out, err := kubectl.Kubectl("get", "ns", "-A")
	l.Info("ns", "ns", out)
	if err != nil {
		return err
	}
	l.Info("install extra crs ", "cr", a.Cfg.extraCrs)
	for _, p := range a.Cfg.extraCrs {
		_, err = kubectl.Kubectl("apply", "-R", "-f", p, "--force")
		if err != nil {
			return err
		}
	}
	_, err = helm.UnInstall(name)
	if err != nil {
		return err
	}
	out, err = kubectl.Kubectl("get", "ns", "-A")
	l.Info("ns", "ns", out)
	if err != nil {
		return err
	}
	out, err = helm.Install(cfgs, name, chartBase, chartBase+"/values.yaml")
	if err != nil {
		return err
	}
	l.Info("chart install", "out", out)
	a.deployAssert.WaitReady(a.Ctx, "alb-operator", a.Cfg.Ns)
	if !a.Cfg.disableDefault {
		a.deployAssert.WaitReady(a.Ctx, a.Cfg.DefaultAlbName, a.Cfg.Ns)
	}
	return nil
}

func (a *AlbK8sCtx) DeployEchoResty() error {
	e := NewEchoResty(a.Cfg.base, a.kubecfg, a.Log)
	return e.Deploy(EchoCfg{Name: "echo-resty", Image: a.Cfg.nginx, Ip: "v4"})
}

func (a *AlbK8sCtx) DeployAlb(yaml string, name string, ns string) {
	a.Kubectl.AssertKubectlApply(yaml)
	Wait(func() (bool, error) {
		alb, err := a.Kubecliet.GetAlbClient().CrdV2beta1().ALB2s(ns).Get(a.Ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		a.Log.Info("status", "alb", alb.Status)
		return alb.Status.State == v2beta1.ALB2StateRunning, nil
	})
}

func (a *AlbK8sCtx) Destory() error {
	if os.Getenv("DEV_MODE") != "true" {
		KindDelete(a.Cfg.KindName)
	}
	return nil
}
