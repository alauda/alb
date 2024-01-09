package gateway

import (
	"context"
	"fmt"

	. "alauda.io/alb2/pkg/operator/controllers/types"
	. "alauda.io/alb2/test/kind/pkg/helper"
	. "alauda.io/alb2/utils/test_utils"
	. "alauda.io/alb2/utils/test_utils/assert"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("gateway", func() {
	Context("standalone mode", func() {
		var ctx context.Context
		var cancel context.CancelFunc
		var actx *AlbK8sCtx
		var cli *K8sClient
		var l logr.Logger
		var kubectl *Kubectl
		var domain string
		var g Gomega
		BeforeEach(func() {
			ctx, cancel = CtxWithSignalAndTimeout(30 * 60)
			actx = NewAlbK8sCtx(ctx, NewAlbK8sCfg().
				UseMockLBSvcCtl([]string{"192.168.0.1"}, []string{"2004::192:168:128:235"}).
				DisableDefaultAlb().
				Build(),
			)
			err := actx.Init()
			l = actx.Log
			l.Error(err, "init env fail")
			Expect(err).Should(BeNil())
			domain = "cpaas.io"
			cfg := actx.Kubecfg
			cli = NewK8sClient(ctx, cfg)
			l = actx.Log
			kubectl = NewKubectl(actx.Cfg.Base, cfg, l)
			g = NewGomegaWithT(GinkgoT())
			_ = g
			l.Info("init ok")
		})

		AfterEach(func() {
			cancel()
			actx.Destroy()
		})

		It("create a stand alone gateay,and curl it", func() {
			err := actx.DeployEchoResty()
			Expect(err).NotTo(HaveOccurred())
			cli.CreateNsIfNotExist("g1")
			kubectl.AssertKubectlApply(`
apiVersion: gateway.networking.k8s.io/v1beta1
kind: Gateway
metadata:
  name: g1 
  namespace: g1 
spec:
  gatewayClassName:  exclusive-gateway
  listeners:
  - name: http
    port: 8234
    protocol: HTTP
    allowedRoutes:
      namespaces:
        from: All
---
apiVersion: gateway.networking.k8s.io/v1beta1
kind: HTTPRoute
metadata:
    name: h1
    namespace: g1
spec:
    parentRefs:
      - kind: Gateway
        namespace: g1
        name: g1
        sectionName: http
    rules:
    - matches:
      - path:
          value: "/foo"
      backendRefs:
        - kind: Service
          name: echo-resty
          namespace: default
          port: 80
          weight: 1
`)
			// get alb from gateay label
			EventuallySuccess(func(g Gomega) {
				gw, err := cli.GetGatewayClient().GatewayV1beta1().Gateways("g1").Get(ctx, "g1", metav1.GetOptions{})
				GNoErr(g, err)
				alb := gw.Labels[fmt.Sprintf(FMT_ALB_REF, domain)]
				GEqual(g, alb != "", true)
				WaitAlbReady(cli, l, ctx, "g1", alb)
				// lbsvc is fake, use pod ip.
				pods, err := cli.GetK8sClient().CoreV1().Pods("g1").List(ctx, metav1.ListOptions{
					LabelSelector: "service_name=alb2-" + alb,
				})
				GNoErr(g, err)
				GEqual(g, len(pods.Items) > 0, true)
				pod := pods.Items[0]
				out, err := actx.Kind.ExecInDocker("curl " + pod.Status.PodIP + ":8234/foo")
				GNoErr(g, err)
				l.Info("out", "out", out)
			}, l)
		})
	})
})
