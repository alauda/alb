package gateway

import (
	"context"
	"fmt"
	_ "fmt"

	a2t "alauda.io/alb2/pkg/apis/alauda/v2beta1"
	. "alauda.io/alb2/pkg/operator/controllers/depl/resources/types"
	. "alauda.io/alb2/pkg/operator/controllers/types"
	. "alauda.io/alb2/test/e2e/framework"
	. "alauda.io/alb2/utils/test_utils"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	appv1 "k8s.io/api/apps/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Operator Gateway", func() {
	var env *OperatorEnv
	var kt *Kubectl
	var kc *K8sClient
	var ctx context.Context
	var g Gomega

	// var kc *K8sClient
	var log logr.Logger
	BeforeEach(func() {
		opt := OperatorEnvCfg{RunOpext: true}
		opt.UseMockLBSvcCtl(append(DEFAULT_V4_IP_POOL, "127.0.0.1"), append(DEFAULT_V6_IP_POOL, "2004::199:168:128:236"))
		env = StartOperatorEnvOrDieWithOpt(opt)
		kt = env.Kt
		kc = env.Kc
		ctx = env.Ctx
		log = env.Log.WithName("intest")
		g = NewGomegaWithT(GinkgoT())
		_ = g
	})

	AfterEach(func() {
		env.Stop()
	})

	GIt("case 2. deploy a alb in standalone gateway mode", func() {
		kc.CreateNsIfNotExist("g1")
		alb := `
apiVersion: crd.alauda.io/v2beta1
kind: ALB2
metadata:
    name: gateway-g1
    namespace: g1
    labels:
        alb.cpaas.io/managed-by: alb-operator
spec:
    address: "127.0.0.1"
    type: "nginx" 
    config:
        networkMode: container
        gateway:
            mode: standalone
            name: "g1"
        replicas: 3
---
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: g1 
  namespace: g1 
  labels:
      alb.cpaas.io/alb-ref: gateway-g1
spec:
  gatewayClassName:  exclusive-gateway
  listeners:
  - name: http
    port: 8234
    protocol: HTTP
    allowedRoutes:
      namespaces:
        from: All
`
		kt.AssertKubectlApply(alb)
		EventuallySuccess(func(g Gomega) {
			log.Info("test")
			// 会创建出有一个global的独享型的gatewayclass
			gc, err := kc.GetGatewayClient().GatewayV1beta1().GatewayClasses().Get(ctx, STAND_ALONE_GATEWAY_CLASS, metav1.GetOptions{})
			g.Expect(err).ShouldNot(HaveOccurred())
			log.Info("gc", "gc", PrettyCr(gc))
			g.Expect(gc.Labels["gatewayclass.cpaas.io/deploy"]).Should(Equal("cpaas.io"))
			g.Expect(gc.Labels["gatewayclass.cpaas.io/type"]).Should(Equal("standalone"))
			a := NewAssertHelperOmgea(ctx, kc, kt, g)
			a.AssertResource(ExpectResource{
				ExpectExist:    []Resource{},
				ExpectNotExist: []Resource{{Ns: "g1", Kind: "feature", Names: []string{"gateway-g1-g1"}}, {Ns: "", Kind: "IngressClass", Names: []string{"gateway-g1"}}},
			})

			a.AssertDeployment("g1", "gateway-g1", ExpectDeployment{
				ExpectContainlerEnv: map[string]map[string]string{"alb2": {
					"NETWORK_MODE":   "container",
					"ALB_ENABLE":     "false",
					"SERVE_INGRESS":  "false",
					"GATEWAY_ENABLE": "true",
					"GATEWAY_MODE":   string(a2t.GatewayModeStandAlone),
					"GATEWAY_NAME":   "g1",
					"GATEWAY_NS":     "g1",
				}},
				Hostnetwork: false,
				Test: func(dep *appv1.Deployment) bool {
					spec := dep.Spec.Template.Spec
					return !spec.HostNetwork &&
						spec.DNSPolicy == "ClusterFirst" &&
						*dep.Spec.Replicas == 3 &&
						spec.Affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution != nil
				},
			})
			// 除了gateway-g1之外不应该部署其他的alb
			alblist, err := kc.GetAlbClient().CrdV2beta1().ALB2s("g1").List(ctx, metav1.ListOptions{})
			g.Expect(err).ShouldNot(HaveOccurred())
			g.Expect(len(alblist.Items)).Should(Equal(1))
		}, log)
	})

	It("case 3. deploy standalone gateway ", func() {
		kc.CreateNsIfNotExist("g1")
		gw := `
apiVersion: gateway.networking.k8s.io/v1
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
`
		kt.AssertKubectlApply(gw)
		EventuallySuccess(func(g Gomega) {
			// 1. should create alb
			albls, err := kc.GetAlbClient().CrdV2beta1().ALB2s("g1").List(ctx, metav1.ListOptions{})
			g.Expect(err).ShouldNot(HaveOccurred())
			g.Expect(len(albls.Items)).Should(Equal(1))
			alb := albls.Items[0]
			log.Info("auto create alb", "alb", PrettyCr(&alb))
			g.Expect(*alb.Spec.Config.Gateway.Mode).Should(Equal(a2t.GatewayModeStandAlone))
			g.Expect(*alb.Spec.Config.Gateway.Name).Should(Equal("g1"))

			// 1.1 should update gateway label
			g1, err := kc.GetGatewayClient().GatewayV1beta1().Gateways("g1").Get(ctx, "g1", metav1.GetOptions{})
			log.Error(err, "get gateway")
			g.Expect(err).ShouldNot(HaveOccurred())
			log.Info("should update gateway label", "gw", PrettyCr(&g1))
			g.Expect(g1.Labels[fmt.Sprintf(FMT_ALB_REF, "cpaas.io")]).Should(Equal(alb.Name))

			// 1.2 should create rbac
			sa, err := kc.GetK8sClient().CoreV1().ServiceAccounts("g1").Get(ctx, fmt.Sprintf(FMT_SA, alb.Name), metav1.GetOptions{})
			g.Expect(err).ShouldNot(HaveOccurred())
			dep, err := kc.GetK8sClient().AppsV1().Deployments("g1").Get(ctx, alb.Name, metav1.GetOptions{})
			g.Expect(err).ShouldNot(HaveOccurred())
			g.Expect(dep.Spec.Template.Spec.ServiceAccountName).Should(Equal(sa.Name))
		}, log)

		// 2. should update gateway status
		EventuallySuccess(func(g Gomega) {
			g1, err := kc.GetGatewayClient().GatewayV1beta1().Gateways("g1").Get(ctx, "g1", metav1.GetOptions{})
			g.Expect(err).ShouldNot(HaveOccurred())
			accept, find := lo.Find(g1.Status.Conditions, func(c metav1.Condition) bool { return c.Type == "Accepted" })
			g.Expect(find).Should(BeTrue())
			log.Info("should update gateway status", "accept", accept)
			g.Expect(accept.Message).Should(ContainSubstring("wait workload ready spec"))
		}, log)

		// 3 change gatewayclass should delete alb
		gatew, err := kc.GetGatewayClient().GatewayV1beta1().Gateways("g1").Get(ctx, "g1", metav1.GetOptions{})
		g.Expect(err).ShouldNot(HaveOccurred())
		gatew.Spec.GatewayClassName = "xxx"
		_, err = kc.GetGatewayClient().GatewayV1beta1().Gateways("g1").Update(ctx, gatew, metav1.UpdateOptions{})
		g.Expect(err).ShouldNot(HaveOccurred())

		EventuallySuccess(func(g Gomega) {
			albls, err := kc.GetAlbClient().CrdV2beta1().ALB2s("g1").List(ctx, metav1.ListOptions{})
			g.Expect(err).ShouldNot(HaveOccurred())
			log.Info("albs", "alb should be delete", PrettyCrs(albls.Items))
			g.Expect(len(albls.Items)).Should(Equal(0))
		}, log)

		gatew, err = kc.GetGatewayClient().GatewayV1beta1().Gateways("g1").Get(ctx, "g1", metav1.GetOptions{})
		g.Expect(err).ShouldNot(HaveOccurred())
		label := gatew.Labels[fmt.Sprintf(FMT_ALB_REF, "cpaas.io")]
		log.Info("should be nil", "label", gatew.Labels, "x", label)
		g.Expect(label).Should(Equal(""))

		log.Info("bring it back")
		gatew, err = kc.GetGatewayClient().GatewayV1beta1().Gateways("g1").Get(ctx, "g1", metav1.GetOptions{})
		g.Expect(err).ShouldNot(HaveOccurred())
		gatew.Spec.GatewayClassName = STAND_ALONE_GATEWAY_CLASS
		_, err = kc.GetGatewayClient().GatewayV1beta1().Gateways("g1").Update(ctx, gatew, metav1.UpdateOptions{})
		g.Expect(err).ShouldNot(HaveOccurred())

		EventuallySuccess(func(g Gomega) {
			albls, err := kc.GetAlbClient().CrdV2beta1().ALB2s("g1").List(ctx, metav1.ListOptions{})
			g.Expect(err).ShouldNot(HaveOccurred())
			log.Info("alb should back", "albs", PrettyCrs(albls.Items))
			g.Expect(len(albls.Items)).Should(Equal(1))
		}, log)

		gatew, err = kc.GetGatewayClient().GatewayV1beta1().Gateways("g1").Get(ctx, "g1", metav1.GetOptions{})
		g.Expect(err).ShouldNot(HaveOccurred())
		log.Info("should not nil", "label", gatew.Labels)
		g.Expect(gatew.Labels[fmt.Sprintf(FMT_ALB_REF, "cpaas.io")]).ShouldNot(Equal(nil))

		// 4. delete gateway alb should be deleted.
		kc.GetGatewayClient().GatewayV1beta1().Gateways("g1").Delete(ctx, "g1", metav1.DeleteOptions{})
		EventuallySuccess(func(g Gomega) {
			albls, err := kc.GetAlbClient().CrdV2beta1().ALB2s("g1").List(ctx, metav1.ListOptions{})
			g.Expect(err).ShouldNot(HaveOccurred())
			g.Expect(len(albls.Items)).Should(Equal(0))
		}, log)
	})
})
