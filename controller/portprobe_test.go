package controller

import (
	"context"
	"os"
	"testing"
	"time"

	"alauda.io/alb2/config"
	"alauda.io/alb2/driver"
	albv1 "alauda.io/alb2/pkg/apis/alauda/v1"
	"alauda.io/alb2/utils/log"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "alauda.io/alb2/utils/test_utils"
)

func TestPortProbe(t *testing.T) {
	base := InitBase()
	l := log.InitKlogV2(log.LogCfg{ToFile: base + "/port-test.log"})
	env := NewEnvtestExt(base, l)
	kcfg := env.AssertStart()
	ctx, _ := context.WithTimeout(context.Background(), time.Minute*10)
	defer env.Stop()
	kt := NewKubectl(base, kcfg, l)
	kc := NewK8sClient(ctx, kcfg)
	_ = kt
	_ = kc
	l.Info("test port probe")

	kt.AssertKubectlApply(`
apiVersion: crd.alauda.io/v2beta1
kind: ALB2
metadata:
    name: a1
    namespace: cpaas-system
spec:
    address: "127.0.0.1"
    type: "nginx"
    config:
       replicas: 1
---
apiVersion: crd.alauda.io/v1
kind: Frontend
metadata:
  labels:
    alb2.cpaas.io/name: a1
  name: a1-00080
  namespace: cpaas-system
spec:
  backendProtocol: ""
  certificate_name: ""
  port: 80
  protocol: tcp
  source:
    name: xx
    namespace: cpaas-system
`)
	ns := "cpaas-system"
	name := "a1"
	ftName := "a1-00080"
	host, err := os.Hostname()
	assert.NoError(t, err)

	cfg := config.DefaultMock()
	cfg.Name = name
	cfg.Ns = ns

	cfg.ALBRunConfig.Controller.Flags.EnablePortProbe = true
	cfg.Controller.PodName = "a"
	config.UseMock(cfg)
	drv, err := driver.InitKubernetesDriverFromCfg(ctx, kcfg)
	assert.NoError(t, err)
	p, err := NewPortProbe(ctx, drv, l, cfg)
	p.listTcpPort = func() (map[int]bool, error) {
		return map[int]bool{80: true}, nil
	}
	assert.NoError(t, err)

	// mock data
	{
		ft, err := kc.GetAlbClient().CrdV1().Frontends(ns).Get(ctx, ftName, metav1.GetOptions{})
		assert.NoError(t, err)
		if ft.Status.Instances == nil {
			ft.Status.Instances = map[string]albv1.Instance{}
		}
		ft.Status.Instances[host] = albv1.Instance{
			Conflict: true,
		}
		_, err = kc.GetAlbClient().CrdV1().Frontends(ns).UpdateStatus(ctx, ft, metav1.UpdateOptions{})
		assert.NoError(t, err)
	}

	alb, err := GetLBConfigFromAlb(*drv, ns, name)
	assert.NoError(t, err)

	// it should mark this port as conflict
	{
		p.WorkerDetectAndMaskConflictPort(alb)
		ft, err := kc.GetAlbClient().CrdV1().Frontends(ns).Get(ctx, ftName, metav1.GetOptions{})
		assert.NoError(t, err)
		l.Info("ft1", "ft", PrettyJson(ft.Status))
		assert.Equal(t, len(ft.Status.Instances), 2)
	}
	// it should update alb status
	{
		// mock a alb pod
		kt.AssertKubectlApply(`
apiVersion: v1
kind: Pod
metadata:
  labels:
    service_name: alb2-a1
  name: a
  namespace: cpaas-system
spec:
  containers:
  - name: x 
    image: x
`)

		assert.Eventually(t, func() bool {
			p.LeaderUpdateAlbPortStatus()
			alb, err := kc.GetAlbClient().CrdV2beta1().ALB2s(ns).Get(ctx, name, metav1.GetOptions{})
			assert.NoError(t, err)
			ft, err := kc.GetAlbClient().CrdV1().Frontends(ns).Get(ctx, ftName, metav1.GetOptions{})
			assert.NoError(t, err)
			l.Info("ft1", "ft", PrettyJson(ft.Status))
			l.Info("alb", "alb", PrettyJson(alb.Status.Detail.Alb))
			return len(ft.Status.Instances) == 1 && alb.Status.Detail.Alb.PortStatus["tcp-80"].Conflict == true
		}, time.Second*30, time.Second*3)
	}
	// when alb pod change we should cleanup the old pod status
	{
		// mock a alb pod
		kt.AssertKubectlApply(`
apiVersion: v1
kind: Pod
metadata:
  labels:
    service_name: alb2-a1
  name: b
  namespace: cpaas-system
spec:
  containers:
  - name: x 
    image: x
`)
		kt.Kubectl("delete po -n cpaas-system a")
		p.listTcpPort = func() (map[int]bool, error) { return map[int]bool{}, nil }
		l.Info("delete pod a, mark port as no confilct")
		assert.Eventually(t, func() bool {
			p.LeaderUpdateAlbPortStatus()
			ft, err := kc.GetAlbClient().CrdV1().Frontends(ns).Get(ctx, ftName, metav1.GetOptions{})
			assert.NoError(t, err)
			l.Info("ft1", "ft", PrettyJson(ft.Status))
			alb, err := kc.GetAlbClient().CrdV2beta1().ALB2s(ns).Get(ctx, name, metav1.GetOptions{})
			assert.NoError(t, err)
			l.Info("alb", "alb", PrettyJson(alb.Status.Detail.Alb))
			return len(alb.Status.Detail.Alb.PortStatus) == 0 && len(ft.Status.Instances) == 0
		}, time.Second*600, time.Second*3)
	}
}
