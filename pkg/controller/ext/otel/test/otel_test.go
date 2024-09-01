package otel_test

import (
	"context"
	"testing"

	"alauda.io/alb2/config"
	"alauda.io/alb2/driver"
	"alauda.io/alb2/ingress"
	albv1 "alauda.io/alb2/pkg/apis/alauda/v1"
	"alauda.io/alb2/pkg/controller/ext/otel"
	. "alauda.io/alb2/pkg/controller/ext/otel/types"
	. "alauda.io/alb2/pkg/utils/test_utils"
	. "alauda.io/alb2/utils"
	. "alauda.io/alb2/utils/test_utils"
	"github.com/lithammer/dedent"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"
	"github.com/xorcare/pointer"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	crcli "sigs.k8s.io/controller-runtime/pkg/client"
)

func TestOtel(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "otel related test")
}

var _ = Describe("otel related test", func() {

	l := GinkgoLog()
	Context("unit", func() {

		It("resolve ip should ok", func() {
			t := GinkgoT()

			addr, err := otel.ResolveDnsIfNeed("http://127.0.0.1:1234/a")
			GinkgoNoErr(err)
			assert.Equal(t, addr, "http://127.0.0.1:1234/a")

			addr, err = otel.ResolveDnsIfNeed("https://localhost:1234/a")
			GinkgoNoErr(err)
			assert.Equal(t, addr, "https://127.0.0.1:1234/a")
		})

		It("merge with default should ok", func() {
			type TCase struct {
				cfg    []*OtelCrConf
				name   string
				expect OtelCrConf
			}
			cases := []TCase{
				{
					cfg:    []*OtelCrConf{},
					expect: *otel.DEFAULT_OTEL.DeepCopy(),
				},
				{
					name: "enable it",
					cfg: []*OtelCrConf{
						{
							Enable: false,
							OtelConf: OtelConf{
								Exporter: &Exporter{
									Collector: &Collector{
										Address:        "http://127.0.0.1:4318",
										RequestTimeout: 1000,
									},
								},
								Sampler: &Sampler{
									Name: "parent_base",
									Options: &SamplerOptions{
										ParentName: pointer.String("trace_id_ratio"),
										Fraction:   pointer.String("0.1"),
									},
								},
								Resource: map[string]string{"x.x": "a1"},
							},
						},
						{
							Enable: true,
							OtelConf: OtelConf{
								Flags: &Flags{
									NoTrustIncomingSpan: true,
								},
							},
						},
					},
					expect: OtelCrConf{
						Enable: true,
						OtelConf: OtelConf{
							Exporter: &Exporter{
								Collector: &Collector{
									Address:        "http://127.0.0.1:4318",
									RequestTimeout: 1000,
								},
								BatchSpanProcessor: &BatchSpanProcessor{
									MaxQueueSize:    2048,
									InactiveTimeout: 2,
								},
							},
							Sampler: &Sampler{
								Name: "parent_base",
								Options: &SamplerOptions{
									ParentName: pointer.String("trace_id_ratio"),
									Fraction:   pointer.String("0.1"),
								},
							},
							Flags: &Flags{
								NoTrustIncomingSpan: true,
								HideUpstreamAttrs:   false,
							},
							Resource: map[string]string{"x.x": "a1"},
						},
					},
				},
				{
					name: "override sampler",
					cfg: []*OtelCrConf{
						{
							Enable: true,
							OtelConf: OtelConf{
								Exporter: &Exporter{
									Collector: &Collector{
										Address:        "http://127.0.0.1:4318",
										RequestTimeout: 1000,
									},
								},
								Sampler: &Sampler{
									Name: "parent_base",
									Options: &SamplerOptions{
										ParentName: pointer.String("trace_id_ratio"),
										Fraction:   pointer.String("0.1"),
									},
								},
								Resource: map[string]string{"x.x": "a1"},
							},
						},
						{
							OtelConf: OtelConf{
								Sampler: &Sampler{
									Name:    "always_off",
									Options: nil,
								},
								Resource: map[string]string{"a": "b"},
							},
						},
						{
							Enable: false,
						},
					},
					expect: OtelCrConf{
						Enable: false,
						OtelConf: OtelConf{
							Exporter: &Exporter{
								Collector: &Collector{
									Address:        "http://127.0.0.1:4318",
									RequestTimeout: 1000,
								},
								BatchSpanProcessor: &BatchSpanProcessor{
									MaxQueueSize:    2048,
									InactiveTimeout: 2,
								},
							},
							Sampler: &Sampler{
								Name: "always_off",
							},
							Flags: &Flags{
								NoTrustIncomingSpan: false,
								HideUpstreamAttrs:   false,
							},
							Resource: map[string]string{"x.x": "a1", "a": "b"},
						},
					},
				},
			}
			for i, c := range cases {
				real, err := otel.MergeWithDefualtJsonPatch(c.cfg, *otel.DEFAULT_OTEL.DeepCopy())
				l.Info("check", "i", i, "name", c.name)
				l.Info("err", "err", err)
				l.Info("real", "real", PrettyJson(real))
				l.Info("expect", "expect", PrettyJson(c.expect))
				GinkgoAssertJsonTEq(real, c.expect, c.name)
			}
		})
	})

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

		It("should generate correct rule when ingress has otel related annotation", func() {
			kt.AssertKubectlApply(dedent.Dedent(`
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: i1 
  namespace: cpaas-system
spec:
  rules:
  - http:
      paths:
      - backend:
          service:
            name: xx
            port:
              number: 8080
        path: /p2
        pathType: ImplementationSpecific
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  annotations:
    nginx.ingress.kubernetes.io/enable-opentelemetry: "true"
  name: i2
  namespace: cpaas-system
spec:
  rules:
  - http:
      paths:
      - backend:
          service:
            name: xx
            port:
              number: 8080
        path: /p2
        pathType: ImplementationSpecific
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  annotations:
    nginx.ingress.kubernetes.io/opentelemetry-trust-incoming-spans: "false"
  name: i3
  namespace: cpaas-system
spec:
  rules:
  - http:
      paths:
      - backend:
          service:
            name: xx
            port:
              number: 8080
        path: /p2
        pathType: ImplementationSpecific
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  annotations:
    alb.ingress.cpaas.io/otel: >
     {
        "enable": true,
        "exporter": {
            "collector": {
                "address": "http://128.0.0.1:4318",
                "request_timeout": 1000
            }
        }
     }
  name: i4
  namespace: cpaas-system
spec:
  rules:
  - http:
      paths:
      - backend:
          service:
            name: xx
            port:
              number: 8080
        path: /p2
        pathType: ImplementationSpecific
`))
			mock := config.DefaultMock()
			drv, err := driver.NewDriver(driver.DrvOpt{Ctx: ctx, Cf: env.GetRestCfg(), Opt: driver.Cfg2opt(mock)})

			ingc := ingress.NewController(drv, drv.Informers, mock, l.WithName("ingress"))
			ing1, err := kc.GetK8sClient().NetworkingV1().Ingresses("cpaas-system").Get(ctx, "i1", metav1.GetOptions{})
			GinkgoNoErr(err)
			ing2, err := kc.GetK8sClient().NetworkingV1().Ingresses("cpaas-system").Get(ctx, "i2", metav1.GetOptions{})
			GinkgoNoErr(err)
			ing3, err := kc.GetK8sClient().NetworkingV1().Ingresses("cpaas-system").Get(ctx, "i3", metav1.GetOptions{})
			GinkgoNoErr(err)
			ing4, err := kc.GetK8sClient().NetworkingV1().Ingresses("cpaas-system").Get(ctx, "i4", metav1.GetOptions{})
			GinkgoNoErr(err)
			ft := &albv1.Frontend{}

			r1, err := ingc.GenerateRule(ing1, crcli.ObjectKey{Namespace: "x", Name: "x"}, ft, 0, 0, "")
			GinkgoNoErr(err)

			r2, err := ingc.GenerateRule(ing2, crcli.ObjectKey{Namespace: "x", Name: "x"}, ft, 0, 0, "")
			GinkgoNoErr(err)

			r3, err := ingc.GenerateRule(ing3, crcli.ObjectKey{Namespace: "x", Name: "x"}, ft, 0, 0, "")
			GinkgoNoErr(err)

			r4, err := ingc.GenerateRule(ing4, crcli.ObjectKey{Namespace: "x", Name: "x"}, ft, 0, 0, "")
			GinkgoNoErr(err)

			l.Info("get rule1", "cr", PrettyCr(r1))
			GinkgoAssertTrue(r1.Spec.Config.Otel == nil, "r1")
			l.Info("get rule2", "cr", PrettyCr(r2))
			GinkgoAssertTrue(r2.Spec.Config.Otel.Enable, "r2")
			l.Info("get rule3", "cr", PrettyCr(r3))
			GinkgoAssertTrue(r3.Spec.Config.Otel.Enable == true, "r3")
			GinkgoAssertTrue(r3.Spec.Config.Otel.Flags.NoTrustIncomingSpan == true, "r3")
			l.Info("get rule4", "cr", PrettyCr(r4))
			GinkgoAssertTrue(r4.Spec.Config.Otel.Enable == true, "r4")
			GinkgoAssertStringEq(r4.Spec.Config.Otel.Exporter.Collector.Address, "http://128.0.0.1:4318", "r4")
		})

		It("should generate policy from alb/ft/rule cr", func() {
			kt.AssertKubectlApply(dedent.Dedent(`
apiVersion: crd.alauda.io/v2beta1
kind: ALB2
metadata:
    name: a1
    namespace: cpaas-system
spec:
    type: "nginx"
    config:
        otel:
            enable: false
            exporter:
                collector:
                    address: "http://127.0.0.1:4318"
                    request_timeout: 1000
            sampler:
                name: "parent_base"
                options:
                    parent_name: "trace_id_ratio"
                    fraction: "0.1"
            resource: 
                "x.x": "a1"
---
apiVersion: crd.alauda.io/v1
kind: Frontend
metadata:
    labels:
        alb2.cpaas.io/name: a1
    name: a1-00080
    namespace: cpaas-system
spec:
    port: 80
    protocol: http
    config:
        otel:
            enable: true
            resource: 
                "a": "b"
---
apiVersion: crd.alauda.io/v1
kind: Rule
metadata:
  labels:
    alb2.cpaas.io/frontend: a1-00080 
    alb2.cpaas.io/name: a1           
  name: a1-00080-1
  namespace: cpaas-system
spec:
  config:
    otel:
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
            `))
			kt.AssertKubectlApply(MockSvc{
				Ns:   "cpaas-system",
				Name: "xx",
				Port: []int{8080},
				Ep:   []string{"192.0.0.1"},
			}.GenYaml())

			l.Info("hello test otel")

			mock := config.DefaultMock()
			drv, err := driver.NewDriver(driver.DrvOpt{Ctx: ctx, Cf: env.GetRestCfg(), Opt: driver.Cfg2opt(mock)})
			GinkgoNoErr(err)
			policy, err := GetPolicy(PolicyGetCtx{Ctx: ctx, Name: "a1", Ns: "cpaas-system", Drv: drv, L: l})
			GinkgoNoErr(err)
			l.Info("policy", "policy", PrettyJson(policy))

			GinkgoAssertJsonEq(
				policy.CommonConfig[*policy.Http.GetPoliciesByPort(80)[0].Config.Otel.OtelRef].Otel.Otel,
				`
		        {
                    "exporter": {
                        "collector": {
                            "address": "http://127.0.0.1:4318",
                            "request_timeout": 1000
                        },
                        "batch_span_processor": {
                              "max_queue_size": 2048,
                              "inactive_timeout": 2
                        }
                    },
                    "sampler": {
                        "name": "parent_base",
                        "options": {
                            "parent_name": "trace_id_ratio",
                            "fraction": "0.1"
                        }
                    },
                    "flags": {
						"hide_upstream_attrs": false,
						"notrust_incoming_span": false,
						"report_http_reqeust_header": false,
						"report_http_response_header": false
					},
                    "resource": {
                        "a": "b",
						"x.x": "a1"
                    }
		        }`,
				"")
			GinkgoAssertJsonEq(
				policy.GetBackendGroup("a1-00080-1"),
				`{
              		"name": "a1-00080-1",
              		"session_affinity_policy": "",
              		"session_affinity_attribute": "",
              		"mode": "http",
              		"backends": [
                  		{
                      		"address": "192.0.0.1",
                      		"otherclusters": false,
                      		"port": 8080,
                      		"svc": "xx",
                      		"ns": "cpaas-system",
                      		"weight": 100
                  		}
              		]
          		}`,
				"")
		})

		// TODO gateway
		It("should generate policy from gateway cr", func() {

		})

	})
})
