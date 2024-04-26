package operator

import (
	"context"
	"fmt"

	. "alauda.io/alb2/controller"
	. "alauda.io/alb2/test/kind/pkg/helper"
	. "alauda.io/alb2/utils/test_utils"
	. "alauda.io/alb2/utils/test_utils/assert"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	crcli "sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("operator", func() {
	Context("container mode alb", func() {
		var ctx context.Context
		var cancel context.CancelFunc
		var actx *AlbK8sCtx
		var cli *K8sClient
		var l logr.Logger
		var kubectl *Kubectl
		var ns string
		BeforeEach(func() {
			ctx, cancel = CtxWithSignalAndTimeout(30 * 60)
			actx = NewAlbK8sCtx(ctx, NewAlbK8sCfg().
				WithWorkerLabels([]string{"name=w1 x=a", "name=w2 x=a", "name=w3"}).
				UseMockLBSvcCtl([]string{"192.168.0.1"}, []string{"2004::192:168:128:235"}).
				WithDefaultAlbName("lb-1").
				Build(),
			)
			err := actx.Init()
			l = actx.Log
			Expect(err).NotTo(HaveOccurred())
			ns = actx.Cfg.Alboperatorcfg.DefaultAlbNS
			cfg := actx.Kubecfg
			cli = NewK8sClient(ctx, cfg)
			l = actx.Log
			kubectl = NewKubectl(actx.Cfg.Base, cfg, l)
			l.Info("init ok")
		})

		AfterEach(func() {
			cancel()
			actx.Destroy()
		})

		It("enablelbsvc and add ft it should add port in lbsvc", func() {
			err := actx.DeployEchoResty()
			Expect(err).NotTo(HaveOccurred())
			kubectl.AssertKubectlApply(`
apiVersion: crd.alauda.io/v2beta1
kind: ALB2
metadata:
    name: c-lb-1
    namespace: cpaas-system
spec:
    address: "127.0.0.1"
    type: "nginx" 
    config:
        vip:
            enableLbSvc: true
        networkMode: container
        nodeSelector:
            name: w1
        projects:
        - ALL_ALL
        replicas: 1
`)
			WaitAlbReady(cli, l, ctx, ns, "c-lb-1")
			kubectl.AssertKubectlApply(`
apiVersion: crd.alauda.io/v1
kind: Frontend
metadata:
  labels:
    alb2.cpaas.io/name: c-lb-1
  name: c-lb-1-12234
  namespace: cpaas-system
spec:
  port: 12234
  protocol: tcp
  serviceGroup:
    services:
    - name: echo-resty
      namespace: default 
      port: 80
      weight: 100
    session_affinity_policy: ""
---
apiVersion: crd.alauda.io/v1
kind: Frontend
metadata:
  labels:
    alb2.cpaas.io/name: c-lb-1
  name: c-lb-1-12553
  namespace: cpaas-system
spec:
  port: 12553
  protocol: udp
  serviceGroup:
    services:
    - name: echo-resty
      namespace: default
      port: 53
      weight: 100
    session_affinity_policy: ""
`)
			Wait(func() (bool, error) {
				svc, err := GetLbSvc(ctx, cli.GetK8sClient(), crcli.ObjectKey{Namespace: ns, Name: "c-lb-1"}, "cpaas.io")
				if k8serrors.IsNotFound(err) {
					l.Info("not found")
					return false, nil
				}
				if err != nil {
					return false, err
				}
				ps := mapset.NewSet(lo.Map(svc.Spec.Ports, func(p v1.ServicePort, i int) string {
					return fmt.Sprintf("%v-%v", p.Port, p.Protocol)
				})...)
				l.Info("wait svc sync", "ports", ps)
				return ps.Contains("12234-TCP") && ps.Contains("12553-UDP"), nil
			})

			kubectl.AssertKubectl("delete ft -n cpaas-system c-lb-1-12234")

			Wait(func() (bool, error) {
				svc, err := GetLbSvc(ctx, cli.GetK8sClient(), crcli.ObjectKey{Namespace: ns, Name: "c-lb-1"}, "cpaas.io")
				if k8serrors.IsNotFound(err) {
					l.Info("not found")
					return false, nil
				}
				if err != nil {
					return false, err
				}
				ps := mapset.NewSet(lo.Map(svc.Spec.Ports, func(p v1.ServicePort, i int) string {
					return fmt.Sprintf("%v-%v", p.Port, p.Protocol)
				})...)
				l.Info("wait svc sync", "ports", ps)
				return !ps.Contains("12234-TCP") && ps.Contains("12553-UDP"), nil
			})
		})

		It("contaner mode alb could deploy on same node", func() {
			// lb-1的alb正常运行
			WaitAlbReady(cli, l, ctx, ns, "lb-1")
			// 部署一个容器网络的alb 在w1上
			kubectl.AssertKubectlApply(`
apiVersion: crd.alauda.io/v2beta1
kind: ALB2
metadata:
    name: c-lb-1
    namespace: cpaas-system
spec:
    address: "127.0.0.1"
    type: "nginx" 
    config:
        vip:
            enableLbSvc: false
        networkMode: container
        nodeSelector:
            name: w1
        projects:
        - ALL_ALL
        replicas: 2
`)
			WaitAlbReady(cli, l, ctx, ns, "c-lb-1")
			// 在部署一个
			kubectl.AssertKubectlApply(`
apiVersion: crd.alauda.io/v2beta1
kind: ALB2
metadata:
    name: c-lb-2
    namespace: cpaas-system
spec:
    address: "127.0.0.1"
    type: "nginx" 
    config:
        vip:
            enableLbSvc: false
        networkMode: container
        nodeSelector:
            x: a
        projects:
        - ALL_ALL
        replicas: 2
`)
			WaitAlbReady(cli, l, ctx, ns, "c-lb-2")
			l.Info(kubectl.AssertKubectl("get po -n cpaas-system -o wide"))
		})
	})
})
