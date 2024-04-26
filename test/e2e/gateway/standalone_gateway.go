package gateway

import (
	"context"
	"fmt"
	"sort"

	. "alauda.io/alb2/test/e2e/framework"
	. "alauda.io/alb2/utils/test_utils"
	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gv1 "sigs.k8s.io/gateway-api/apis/v1"
)

var _ = Describe("StandAloneGateway", func() {
	var f *GatewayF
	var env *Env
	var ctx context.Context
	var kt *Kubectl
	var kc *K8sClient
	var l logr.Logger
	var albName string
	var albNs string
	var g Gomega

	BeforeEach(func() {
		albName = "alb-dev-xx"
		albNs = "g1"
		opt := AlbEnvOpt{
			BootYaml: `
apiVersion: crd.alauda.io/v2beta1
kind: ALB2
metadata:
    name: alb-dev-xx
    namespace: g1 
spec:
    address: ""
    type: "nginx"
    config:
        gateway:
            enable: true
            mode: standalone
            name: "g1"
---
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
    name: g1
    namespace: g1
    labels:
      alb.cpaas.io/gateway-ref: alb-dev-xx
spec:
    gatewayClassName:  exclusive-gateway
    listeners:
    - name: http
      port: 8234
      protocol: HTTP
      hostname: a.com
      allowedRoutes:
        namespaces:
          from: All
`,
			Ns:       albNs,
			Name:     albName,
			StartAlb: true,
		}
		opt.UseMockSvcWithHost(DEFAULT_V4_IP_POOL, DEFAULT_V6_IP_POOL, []string{"a.com"})
		f, env = NewGatewayF(opt)
		f.InitProductNs("t1", "")
		f.InitDefaultSvc("svc-1", []string{"192.168.1.1", "192.168.2.2"})
		ctx = env.Ctx
		kt = f.Kubectl
		kc = f.K8sClient
		l = env.Log
		g = NewGomegaWithT(GinkgoT())
		_ = ctx
		_ = kt
		_ = kc
		_ = l
		_ = g
	})

	AfterEach(func() {
		env.Stop()
	})

	// 当创建了一个独享模式的的gateway时,应该去找对应的alb,并把alb的获取到的地址写在gateway的地址上
	GIt("when i create a standalone mode gateway, it should create alb related resource in this ns, and should reconcile this gateway normally", func() {
		f.AssertKubectlApply(`
apiVersion: gateway.networking.k8s.io/v1
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
      name: svc-1
      namespace: t1
      port: 80
      weight: 1
`)
		l.Info("wait nginx")
		f.WaitNginxConfigStr("listen.*8234")
		l.Info("wait nginx ok")
		// gateway的status 应该是从 notready ，然后deployment ready 然后alb ready 然后才ready的
		EventuallySuccess(func(o Gomega) {
			l.Info("wait gateway ok")
			g, err := kc.GetGatewayClient().GatewayV1().Gateways("g1").Get(ctx, "g1", metav1.GetOptions{})
			o.Expect(err).ShouldNot(HaveOccurred())
			l.Info("gateway status", "status", PrettyJson(g.Status))
			accept, find := lo.Find(g.Status.Conditions, func(c metav1.Condition) bool { return c.Type == "Accepted" })
			o.Expect(find).Should(BeTrue())
			o.Expect(accept.Message).Should(ContainSubstring("wait workload ready spec"))
		}, l)

		gw, err := kc.GetGatewayClient().GatewayV1().Gateways("g1").Get(ctx, "g1", metav1.GetOptions{})
		g.Expect(err).ShouldNot(HaveOccurred())
		depl := gw.Labels["alb.cpaas.io/alb-ref"]
		MakeDeploymentReady(ctx, kc.GetK8sClient(), gw.Namespace, depl)
		EventuallySuccess(func(o Gomega) {
			g, err := kc.GetGatewayClient().GatewayV1().Gateways("g1").Get(ctx, "g1", metav1.GetOptions{})
			o.Expect(err).ShouldNot(HaveOccurred())
			l.Info("gateway status should ready", "status", PrettyJson(g.Status))
			accept, find := lo.Find(g.Status.Conditions, func(c metav1.Condition) bool { return c.Type == "Accepted" })
			o.Expect(find).Should(BeTrue())
			o.Expect(accept.Reason).Should(Equal("Ready"))
			address := lo.Map(g.Status.Addresses, func(a gv1.GatewayStatusAddress, i int) string {
				return a.Value
			})

			expectAddress := []string{"199.168.0.1", "2004::199:168:128:235", "a.com"}
			StringArrayEq(o, address, expectAddress)
		}, l)

		l.Info("gateway is ready now")
		EventuallySuccess(func(o Gomega) {
			g, err := kc.GetGatewayClient().GatewayV1().Gateways("g1").Get(ctx, "g1", metav1.GetOptions{})
			o.Expect(err).ShouldNot(HaveOccurred())
			l.Info("gateway status", "status", PrettyJson(g.Status.Addresses))
			address := lo.Map(g.Status.Addresses, func(a gv1.GatewayStatusAddress, i int) string {
				return a.Value
			})
			StringArrayEq(o, address, []string{"199.168.0.1", "2004::199:168:128:235", "a.com"})
		}, l)

		EventuallySuccess(func(g Gomega) {
			l.Info("update gateway address")
			// update alb address should  update gateway
			cli := kc.GetAlbClient().CrdV2beta1().ALB2s(albNs)
			alb, err := cli.Get(ctx, albName, metav1.GetOptions{})
			GNoErr(g, err)
			alb.Spec.Address = "127.0.0.2,127.0.0.3"
			_, err = cli.Update(ctx, alb, metav1.UpdateOptions{})
			if err != nil {
				l.Error(err, "")
				GNoErr(g, fmt.Errorf("update alb address fail %v", err))
			}
		}, l)

		EventuallySuccess(func(o Gomega) {
			g, err := kc.GetGatewayClient().GatewayV1().Gateways("g1").Get(ctx, "g1", metav1.GetOptions{})
			o.Expect(err).ShouldNot(HaveOccurred())
			l.Info("gateway status", "status", PrettyJson(g.Status))
			address := lo.Map(g.Status.Addresses, func(a gv1.GatewayStatusAddress, i int) string {
				return a.Value
			})
			StringArrayEq(o, address, []string{"127.0.0.2", "127.0.0.3", "199.168.0.1", "2004::199:168:128:235", "a.com"})
		}, l)
	})
})

func StringArrayEq(o Gomega, left []string, right []string) {
	sort.Strings(left)
	sort.Strings(right)
	o.Expect(cmp.Diff(left, right)).Should(Equal(""))
}
