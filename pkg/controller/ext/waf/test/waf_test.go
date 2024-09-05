package waf

import (
	"context"
	"fmt"
	"strings"
	"testing"

	lu "alauda.io/alb2/utils"
	"github.com/lithammer/dedent"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"alauda.io/alb2/config"
	"alauda.io/alb2/driver"
	ingc "alauda.io/alb2/ingress"
	albv1 "alauda.io/alb2/pkg/apis/alauda/v1"
	. "alauda.io/alb2/pkg/controller/ngxconf"
	ptu "alauda.io/alb2/pkg/utils/test_utils"
	. "alauda.io/alb2/utils/test_utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	_ = fmt.Println
	l = GinkgoLog()
)

func TestWaf(t *testing.T) {
	t.Logf("ok")
	RegisterFailHandler(Fail)
	RunSpecs(t, "nginx config")
}

var _ = Describe("waf", func() {
	Context("albenv", func() {
		base := InitBase()
		var env *EnvtestExt
		var kt *Kubectl
		var kc *K8sClient
		var ctx context.Context
		var ctx_cancel context.CancelFunc
		BeforeEach(func() {
			env = NewEnvtestExt(InitBase(), GinkgoLog())
			env.AssertStart()
			kt = env.Kubectl()
			ctx, ctx_cancel = context.WithCancel(context.Background())
			kc = NewK8sClient(ctx, env.GetRestCfg())
			_ = base
			_ = l
			_ = kt
			_ = kc
		})
		AfterEach(func() {
			ctx_cancel()
			env.Stop()
		})

		It("ingress sync show work", func() {
			l.Info("hello")
			ingYaml := `
            apiVersion: networking.k8s.io/v1
            kind: Ingress
            metadata:
              annotations:
                nginx.ingress.kubernetes.io/enable-modsecurity: "true"
                nginx.ingress.kubernetes.io/enable-owasp-core-rules: "true"
                nginx.ingress.kubernetes.io/modsecurity-transaction-id: "$request_id"
                alb.modsecurity.cpaas.io/cmref: "a/b#c"
                alb.modsecurity.cpaas.io/use-recommand: "false"
                nginx.ingress.kubernetes.io/modsecurity-snippet: |
                      a=b;
              name: demo
              namespace: cpaas-system
            spec:
              rules:
              - http:
                  paths:
                  - path: /p2
                    pathType: ImplementationSpecific
                    backend:
                      service:
                        name: x2
                        port:
                          number: 8080
                  - path: /p1
                    pathType: ImplementationSpecific
                    backend:
                      service:
                        name: x1
                        port:
                          number: 8080
            `
			kt.AssertKubectlApply(ingYaml)
			gcf := config.DefaultMock()
			drv, err := driver.NewDriver(driver.DrvOpt{
				Ctx: ctx,
				Cf:  env.GetRestCfg(),
				Opt: driver.Cfg2opt(gcf),
			})
			GinkgoNoErr(err)
			ingctl := ingc.NewController(drv, drv.Informers, gcf, l.WithName("ingress"))
			ns := "cpaas-system"
			ft := &albv1.Frontend{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "xx",
					Namespace: ns,
				},
				Spec: albv1.FrontendSpec{
					Port: albv1.PortNumber(1000),
				},
			}
			ing, err := kc.GetK8sClient().NetworkingV1().Ingresses(ns).Get(ctx, "demo", metav1.GetOptions{})
			GinkgoNoErr(err)
			r, err := ingctl.GenerateRule(ing, client.ObjectKey{Namespace: "", Name: ""}, ft, 0, 0, "")
			GinkgoNoErr(err)
			l.Info("waf", "rule", PrettyCr(r))
			waf := r.Spec.Config.ModeSecurity
			GinkgoAssertJsonEq(waf, `
			{
               "cmRef": "a/b#c",
               "enable": true,
               "useRecommand": false,
               "transactionId": "$request_id",
               "useCoreRules": true
            }`, "")
			GinkgoAssertStringEq(r.Annotations["nginx.ingress.kubernetes.io/modsecurity-snippet"], "a=b;\n", "")
		})

		It("rule should work", func() {
			kt.AssertKubectlApply(dedent.Dedent(`
            apiVersion: crd.alauda.io/v2
            kind: ALB2
            metadata:
                name: waf
                namespace: cpaas-system
            spec:
                type: "nginx"
            ---
            apiVersion: crd.alauda.io/v1
            kind: Frontend
            metadata:
                labels:
                    alb2.cpaas.io/name: waf
                name: waf-00080
                namespace: cpaas-system
            spec:
                port: 80
                protocol: http
            ---
            apiVersion: crd.alauda.io/v1
            kind: Rule
            metadata:
              labels:
                alb2.cpaas.io/frontend: waf-00080 
                alb2.cpaas.io/name: waf
              name: waf-00080-no-cfg
              namespace: cpaas-system
            spec:
              dslx:
              - type: URL
                values:
                - - STARTS_WITH
                  - /xxxx
              serviceGroup:
                  services:
                  - name: xx
                    namespace: cpaas-system
                    port: 8080
                    weight: 100
            ---
            apiVersion: crd.alauda.io/v1
            kind: Rule
            metadata:
              annotations:
                nginx.ingress.kubernetes.io/modsecurity-snippet: |
                  a=b;
              labels:
                alb2.cpaas.io/frontend: waf-00080 
                alb2.cpaas.io/name: waf
              name: waf-00080-with-annotation
              namespace: cpaas-system
            spec:
              config:
                  modsecurity:
                    enable: true
                    useCoreRules: true
              dslx:
              - type: URL
                values:
                - - STARTS_WITH
                  - /a1
              serviceGroup:
                  services:
                  - name: xx
                    namespace: cpaas-system
                    port: 8080
                    weight: 100
            ---
            apiVersion: crd.alauda.io/v1
            kind: Rule
            metadata:
              labels:
                alb2.cpaas.io/frontend: waf-00080 
                alb2.cpaas.io/name: waf
              name: waf-00080-with-trans-id
              namespace: cpaas-system
            spec:
              config:
                  modsecurity:
                    enable: true
                    transactionId: "$request_id"
              dslx:
              - type: URL
                values:
                - - STARTS_WITH
                  - /a1
              serviceGroup:
                  services:
                  - name: xx
                    namespace: cpaas-system
                    port: 8080
                    weight: 100
            ---
            apiVersion: crd.alauda.io/v1
            kind: Rule
            metadata:
              labels:
                alb2.cpaas.io/frontend: waf-00080 
                alb2.cpaas.io/name: waf
              name: waf-00080-disable
              namespace: cpaas-system
            spec:
              config:
                  modsecurity:
                    enable: false
              dslx:
              - type: URL
                values:
                - - STARTS_WITH
                  - /a1
              serviceGroup:
                  services:
                  - name: xx
                    namespace: cpaas-system
                    port: 8080
                    weight: 100
            ---
            apiVersion: crd.alauda.io/v1
            kind: Rule
            metadata:
              labels:
                alb2.cpaas.io/frontend: waf-00080 
                alb2.cpaas.io/name: waf
              name: waf-00080-with-ingress-2
              namespace: cpaas-system
            spec:
              source:
                name: hello-world
                namespace: default
                type: ingress
              config:
                  modsecurity:
                    enable: true
              dslx:
              - type: URL
                values:
                - - STARTS_WITH
                  - /a1
              serviceGroup:
                  services:
                  - name: xx
                    namespace: cpaas-system
                    port: 8080
                    weight: 100
            ---
            apiVersion: crd.alauda.io/v1
            kind: Rule
            metadata:
              labels:
                alb2.cpaas.io/frontend: waf-00080 
                alb2.cpaas.io/name: waf
              name: waf-00080-with-ingress-1
              namespace: cpaas-system
            spec:
              source:
                name: hello-world
                namespace: default
                type: ingress
              config:
                  modsecurity:
                    enable: true
              dslx:
              - type: URL
                values:
                - - STARTS_WITH
                  - /a1
              serviceGroup:
                  services:
                  - name: xx
                    namespace: cpaas-system
                    port: 8080
                    weight: 100
            ---
            apiVersion: v1
            kind: ConfigMap
            metadata:
              name: waf
              namespace: cpaas-system
            data:
              a: |
                xx=xx
            ---
            apiVersion: crd.alauda.io/v1
            kind: Rule
            metadata:
              labels:
                alb2.cpaas.io/frontend: waf-00080 
                alb2.cpaas.io/name: waf
              name: waf-00080-with-cmref
              namespace: cpaas-system
            spec:
              config:
                  modsecurity:
                    enable: true
                    cmRef: "cpaas-system/waf#a"
              dslx:
              - type: URL
                values:
                - - STARTS_WITH
                  - /a1
              serviceGroup:
                  services:
                  - name: xx
                    namespace: cpaas-system
                    port: 8080
                    weight: 100
            ---
            apiVersion: crd.alauda.io/v1
            kind: Rule
            metadata:
              labels:
                alb2.cpaas.io/frontend: waf-00080 
                alb2.cpaas.io/name: waf
              name: waf-00080-1
              namespace: cpaas-system
            spec:
              config:
                  modsecurity:
                    enable: true
                    useCoreRules: true
              dslx:
              - type: URL
                values:
                - - STARTS_WITH
                  - /a1
              serviceGroup:
                  services:
                  - name: xx
                    namespace: cpaas-system
                    port: 8080
                    weight: 100
            `))

			mock := config.DefaultMock()
			drv, err := driver.NewDriver(driver.DrvOpt{Ctx: ctx, Cf: env.GetRestCfg(), Opt: driver.Cfg2opt(mock)})
			GinkgoNoErr(err)
			policy, tmpl, err := ptu.GetPolicyAndNgx(ptu.PolicyGetCtx{
				Ctx: ctx, Name: "waf", Ns: "cpaas-system", Drv: drv, L: l,
				Cfg: mock,
			})
			GinkgoNoErr(err)
			ngxconf, err := RenderNginxConfigEmbed(*tmpl)
			GinkgoNoErr(err)

			p := ptu.NgxPolicy(*policy)

			l.Info("ngx", "raw", ngxconf, "p", lu.PrettyJson(policy))
			// nginx.conf should has location, policy should has to location
			loc := ptu.FindNamedHttpLocationRaw(ngxconf, "80", "@waf_rule_waf-00080-1")
			loc_str := ptu.DumpNgxBlockEq(loc.GetBlock())
			GinkgoAssertTrue(strings.Contains(loc_str, "modsecurity on;"), "")
			GinkgoAssertTrue(strings.Contains(loc_str, "owasp-modsecurity-crs"), "")
			GinkgoAssertTrue(*p.FindHttpPolicyOnly("waf-00080-1").ToLocation == "waf_rule_waf-00080-1", "")

			// rule with transactionId
			loc_str = ptu.DumpNgxBlockEq(ptu.FindNamedHttpLocationRaw(ngxconf, "80", "@waf_rule_waf-00080-with-trans-id").GetBlock())
			l.Info("trans_id loc", "loc", loc_str)
			GinkgoAssertTrue(strings.Contains(loc_str, `modsecurity_transaction_id "$request_id";`), "")

			// no cfg no tolocation
			GinkgoAssertTrue(p.FindHttpPolicyOnly("waf-00080-no-cfg").ToLocation == nil, "")
			// rule from ingress share key
			GinkgoAssertTrue(*p.FindHttpPolicyOnly("waf-00080-with-ingress-1").ToLocation == "waf_ing_default_hello-world", "")
			GinkgoAssertTrue(*p.FindHttpPolicyOnly("waf-00080-with-ingress-2").ToLocation == "waf_ing_default_hello-world", "")

			// rule use cmref
			loc = ptu.FindNamedHttpLocationRaw(ngxconf, "80", "@waf_rule_waf-00080-with-cmref")
			loc_str = ptu.DumpNgxBlockEq(loc.GetBlock())
			l.Info("loc", "loc", loc_str)
			GinkgoAssertTrue(strings.Contains(loc_str, "xx=xx"), "")
			GinkgoAssertStringEq(*p.FindHttpPolicyOnly("waf-00080-with-cmref").ToLocation, "waf_rule_waf-00080-with-cmref", "")

			// different config use different location
			loc = ptu.FindNamedHttpLocationRaw(ngxconf, "80", "@waf_rule_waf-00080-with-annotation")
			loc_str = ptu.DumpNgxBlockEq(loc.GetBlock())
			l.Info("loc", "loc", loc_str)
			GinkgoAssertTrue(strings.Contains(loc_str, "a=b"), "")
			GinkgoAssertStringEq(*p.FindHttpPolicyOnly("waf-00080-with-annotation").ToLocation, "waf_rule_waf-00080-with-annotation", "")

			// disabled rule no tolocation
			GinkgoAssertTrue(p.FindHttpPolicyOnly("waf-00080-disable").ToLocation == nil, "")

			// enable waf in ft
			kt.AssertKubectlApply(dedent.Dedent(`
            apiVersion: crd.alauda.io/v1
            kind: Frontend
            metadata:
                labels:
                    alb2.cpaas.io/name: waf
                name: waf-00080
                namespace: cpaas-system
            spec:
                port: 80
                protocol: http
                config:
                  modsecurity:
                    enable: true
            `))

			policy, tmpl, err = ptu.GetPolicyAndNgx(ptu.PolicyGetCtx{
				Ctx: ctx, Name: "waf", Ns: "cpaas-system", Drv: drv, L: l,
				Cfg: mock,
			})
			GinkgoNoErr(err)
			ngxconf, err = RenderNginxConfigEmbed(*tmpl)
			GinkgoNoErr(err)

			p = ptu.NgxPolicy(*policy)

			l.Info("ngx", "raw", ngxconf, "p", lu.PrettyJson(policy))
			// no cfg use cfg from ft
			GinkgoAssertStringEq(*p.FindHttpPolicyOnly("waf-00080-no-cfg").ToLocation, "waf_ft_waf-00080", "")
			// other keep same
			loc = ptu.FindNamedHttpLocationRaw(ngxconf, "80", "@waf_rule_waf-00080-with-annotation")
			loc_str = ptu.DumpNgxBlockEq(loc.GetBlock())
			GinkgoAssertTrue(strings.Contains(loc_str, "a=b"), "")

			// enable waf in alb
			kt.AssertKubectlApply(dedent.Dedent(`
            apiVersion: crd.alauda.io/v1
            kind: Frontend
            metadata:
                labels:
                    alb2.cpaas.io/name: waf
                name: waf-00080
                namespace: cpaas-system
            spec:
                port: 80
                protocol: http
            ---
            apiVersion: crd.alauda.io/v2
            kind: ALB2
            metadata:
                name: waf
                namespace: cpaas-system
            spec:
                type: "nginx"
                config:
                  modsecurity:
                    enable: true
            `))

			policy, tmpl, err = ptu.GetPolicyAndNgx(ptu.PolicyGetCtx{
				Ctx: ctx, Name: "waf", Ns: "cpaas-system", Drv: drv, L: l,
				Cfg: mock,
			})
			GinkgoNoErr(err)
			ngxconf, err = RenderNginxConfigEmbed(*tmpl)
			GinkgoNoErr(err)

			p = ptu.NgxPolicy(*policy)

			l.Info("ngx", "raw", ngxconf, "p", lu.PrettyJson(policy))
			// no cfg use cfg from ft
			GinkgoAssertStringEq(*p.FindHttpPolicyOnly("waf-00080-no-cfg").ToLocation, "waf_alb_waf-00080", "")
			// other keep same
			loc = ptu.FindNamedHttpLocationRaw(ngxconf, "80", "@waf_rule_waf-00080-with-annotation")
			loc_str = ptu.DumpNgxBlockEq(loc.GetBlock())
			GinkgoAssertTrue(strings.Contains(loc_str, "a=b"), "")
		})
	})
})
