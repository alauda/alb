package helper

import (
	"context"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/ztrue/tracerr"
	"k8s.io/client-go/rest"
	"k8s.io/utils/env"

	"alauda.io/alb2/utils/log"
	. "alauda.io/alb2/utils/test_utils"
	. "alauda.io/alb2/utils/test_utils/assert"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type AlbK8sCfgOption struct {
	kindcfg        *KindConfig
	base           *string
	chart          *string
	chartoverwrite []string
	name           *string // alb name
	ns             *string
	version        *string
}

func NewAlbK8sCfg(chart string, name string) *AlbK8sCfgOption {
	return &AlbK8sCfgOption{
		chart: &chart,
		name:  &name,
	}
}

func (c *AlbK8sCfgOption) Build() AlbK8sCfg {

	if c.base == nil {
		base := InitBase()
		c.base = &base
	}

	if c.name == nil {
		name := "global-alb2"
		c.name = &name
	}

	// add branch prefix
	kindName := fmt.Sprintf("%s-%d", *c.name, time.Now().Unix())

	if c.kindcfg == nil {
		c.kindcfg = &KindConfig{
			Base:  *c.base,
			Name:  kindName,
			Image: "kindest/node:v1.24.3",
			ClusterYaml: `
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
networking:
  ipFamily: ipv4
  apiServerAddress: "127.0.0.1"
nodes:
- role: control-plane
- role: worker
containerdConfigPatches:
- |-
   [plugins."io.containerd.grpc.v1.cri".registry.mirrors."registry.alauda.cn:60080"] endpoint = ["http://registry.alauda.cn:60080"]
`,
		}
	}

	if c.chartoverwrite == nil {
		c.chartoverwrite = []string{
			fmt.Sprintf(`
operatorDeployMode: "deployment"
projects: [ALL_ALL]
displayName: ""
global:
    labelBaseDomain: cpaas.io
    namespace: cpaas-system
    registry:
      address: registry.alauda.cn:60080
loadbalancerName: %s
replicas: 1
`, *c.name)}
	}
	if c.ns == nil {
		ns := "cpaas-system"
		c.ns = &ns
	}
	extracrs := []string{}
	if env.GetString("ALB_ROOT", "") != "" {
		albroot := env.GetString("ALB_ROOT", "")
		extracrs = append(extracrs, path.Join(albroot, "deploy", "resource", "rbac"))
		extracrs = append(extracrs, path.Join(albroot, "scripts", "yaml", "crds", "extra", "v1"))
	}
	if env.GetString("ALB_EXTRA_CRS", "") != "" {
		extracr := env.GetString("ALB_EXTRA_CRS", "")
		extracrs = append(extracrs, extracr)
	}
	if c.version == nil {
		versions := strings.Split(*c.chart, ":")
		version := versions[len(versions)-1]
		fmt.Println(version)
		c.version = &version
	}

	return AlbK8sCfg{
		kindcfg:        *c.kindcfg,
		base:           *c.base,
		chart:          *c.chart,
		chartoverwrite: c.chartoverwrite,
		Name:           *c.name,
		Ns:             *c.ns,
		nginx:          "registry.alauda.cn:60080/acp/alb-nginx:v3.12.2",
		extraCrs:       extracrs,
		version:        *c.version,
	}
}

type AlbK8sCfg struct {
	kindcfg        KindConfig
	base           string
	chart          string
	chartoverwrite []string
	Name           string
	Ns             string
	version        string
	nginx          string
	extraCrs       []string
}

type AlbK8sExt struct {
	kind         *KindExt
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
	kubecfg, err := a.kind.GetConfig()
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

	albchart := NewAlbChart(helm)
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
	a.kind = kind
	l.Info("kind", "name", kind.Name)
	a.initExt()
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

	if err := a.DeployAlb(); err != nil {
		return err
	}

	return nil
}

// 给定kubecfg部署一个alb
// 包括同步镜像之类的
func (a *AlbK8sCtx) DeployAlb() error {
	l := a.Log
	ctx := a.Ctx
	ns := a.Cfg.Ns
	name := a.Cfg.Name
	kc := a.Kubecliet
	version := a.Cfg.version

	alb, err := kc.GetAlbClient().CrdV2beta1().ALB2s(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil && !kerr.IsNotFound(err) {
		return err
	}
	if err == nil {
		// we found a alb. if its version is what we want to deploy. ignore it.
		if alb.Status.Detail.Versions.Version != version {
			return fmt.Errorf("version not same %v %v", alb.Status.Detail.Versions.Version, version)
		}
		l.Info("alb deployed skip install", "alb", PrettyCr(alb))
		a.deployAssert.WaitReady(ctx, a.Cfg.Name, a.Cfg.Ns)
		return nil
	}

	if err := a.DeployOperator(); err != nil {
		return err
	}

	return nil
}

func (a *AlbK8sCtx) DeployOperator() error {
	chart := a.Cfg.chart
	l := a.Log
	helm := a.helm
	ac := a.albChart
	d := a.docker
	kind := a.kind
	kc := a.Kubecliet
	ns := a.Cfg.Ns
	kubectl := a.Kubectl
	name := a.Cfg.Name
	cfgs := a.Cfg.chartoverwrite

	chartBase, err := ac.Pull(chart)
	if err != nil {
		return err
	}
	images, err := ac.ListImage(chart)
	if err != nil {
		return err
	}
	err = d.Pull(images...)
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
	a.deployAssert.WaitReady(a.Ctx, a.Cfg.Name, a.Cfg.Ns)
	return nil
}

func (a *AlbK8sCtx) DeployApp() error {
	e := NewEchoResty(a.Cfg.base, a.kubecfg, a.Log)
	return e.Deploy(EchoCfg{Name: "echo-resty", Image: a.Cfg.nginx, Ip: "v4"})
}
