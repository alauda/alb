package ingress

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	av1 "alauda.io/alb2/pkg/apis/alauda/v1"
	. "alauda.io/alb2/test/e2e/framework"
	. "alauda.io/alb2/utils/test_utils"
	. "alauda.io/alb2/utils/test_utils/assert"
	"github.com/go-logr/logr"
	"github.com/onsi/ginkgo"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var _ = ginkgo.Describe("Ingress", func() {
	var f *Framework
	var l logr.Logger
	var albname string
	var albns string

	ginkgo.BeforeEach(func() {
		deployCfg := Config{InstanceMode: true, RestCfg: KUBE_REST_CONFIG, Project: []string{"project1"}}
		f = NewAlb(deployCfg)
		f.InitProductNs("alb-test", "project1")
		f.InitDefaultSvc("svc-default", []string{"192.168.1.1", "192.168.2.2"})
		f.Init()
		l = GinkgoLog()
		albname = f.AlbName
		albns = f.AlbNs
		_ = l
	})

	ginkgo.AfterEach(func() {
		f.Destroy()
		f = nil
	})

	GIt("empty test", func() {
		Logf("ok")
	})

	GIt("should compatible with redirect ingress", func() {
		ns := f.GetProductNs()
		// should generate rule and policy when use ingress.backend.port.number even svc not exist.
		pathType := networkingv1.PathTypeImplementationSpecific
		_, err := f.GetK8sClient().NetworkingV1().Ingresses(ns).Create(context.Background(), &networkingv1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ns,
				Name:      "redirect",
				Annotations: map[string]string{
					"nginx.ingress.kubernetes.io/temporal-redirect": "/console-platform/portal",
				},
			},
			Spec: networkingv1.IngressSpec{
				Rules: []networkingv1.IngressRule{
					{
						IngressRuleValue: networkingv1.IngressRuleValue{
							HTTP: &networkingv1.HTTPIngressRuleValue{
								Paths: []networkingv1.HTTPIngressPath{
									{
										Path:     "/&",
										PathType: &pathType,
										Backend: networkingv1.IngressBackend{
											Service: &networkingv1.IngressServiceBackend{
												Name: "none",
												Port: networkingv1.ServiceBackendPort{Number: 8080},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}, metav1.CreateOptions{})

		assert.NoError(ginkgo.GinkgoT(), err)
		f.WaitNginxConfigStr("listen.*80")

		rules := f.WaitIngressRule("redirect", ns, 1)
		rule := rules[0]
		ruleName := rules[0].Name

		assert.Equal(ginkgo.GinkgoT(), rule.Spec.ServiceGroup.Services[0].Port, 8080)

		f.WaitPolicy(func(policyRaw string) bool {
			fmt.Printf("policyRaw %s", policyRaw)
			hasRule := PolicyHasRule(policyRaw, 80, ruleName)
			hasPod := PolicyHasBackEnds(policyRaw, ruleName, `[]`)
			return hasRule && hasPod
		})
	})

	ginkgo.Context("basic ingress", func() {
		GIt("should translate ingress to rule and generate nginx config,policy.new", func() {
			ns := f.GetProductNs()
			name := "ingress-a"

			f.CreateIngress(ns, name, "/a", "svc-default", 80)

			f.WaitNginxConfigStr("listen.*80")

			rules := f.WaitIngressRule(name, ns, 1)
			rule := rules[0]
			ruleName := rule.Name

			f.WaitPolicy(func(policyRaw string) bool {
				hasRule := PolicyHasRule(policyRaw, 80, ruleName)
				hasPod := PolicyHasBackEnds(policyRaw, ruleName, `[map[address:192.168.1.1 otherclusters:false port:80 weight:50] map[address:192.168.2.2 otherclusters:false port:80 weight:50]]`)
				return hasRule && hasPod
			})

		})

		GIt("should get correct backend port when ingress use port.name", func() {
			// give a svc.port tcp-81 which point to pod name pod-tcp-112 which point to pod container port 112
			// if ingress.backend.port.name=tcp-81, the backend should be xxxx:112
			ns := f.GetProductNs()
			ingressCase := IngressCase{
				Namespace: ns,
				Name:      "svc2",
				SvcPort: map[string]struct {
					Protocol   corev1.Protocol
					Port       int32
					Target     intstr.IntOrString
					TargetPort int32
					TargetName string
				}{
					"tcp-81": {
						Protocol:   corev1.ProtocolTCP,
						Port:       81,
						Target:     intstr.IntOrString{Type: intstr.String, StrVal: "pod-tcp-112"},
						TargetName: "pod-tcp-112",
						TargetPort: 112,
					},
				},
				Eps: []string{"192.168.3.1", "192.168.3.2"},
				Ingress: struct {
					Name string
					Host string
					Path string
					Port intstr.IntOrString
				}{
					Name: "svc-ingress",
					Host: "host-a",
					Path: "/a",
					Port: intstr.IntOrString{Type: intstr.String, StrVal: "tcp-81"},
				},
			}

			f.InitIngressCase(ingressCase)
			f.WaitNginxConfigStr("listen.*80")
			rules := f.WaitIngressRule(ingressCase.Ingress.Name, ingressCase.Namespace, 1)
			rule := rules[0]
			ruleName := rule.Name
			Logf("created rule %+v", rule)
			assert.Equal(ginkgo.GinkgoT(), rule.Spec.ServiceGroup.Services[0].Port, 81)
			f.WaitPolicy(func(policyRaw string) bool {
				hasRule := PolicyHasRule(policyRaw, 80, ruleName)
				hasPod := PolicyHasBackEnds(policyRaw, ruleName, `[map[address:192.168.3.1 otherclusters:false port:112 weight:50] map[address:192.168.3.2 otherclusters:false port:112 weight:50]]`)
				return hasRule && hasPod
			})
		})

		GIt("should work when ingress has longlong name", func() {
			ns := f.GetProductNs()
			_ = ns
			name := "ingress-aaa" + strings.Repeat("a", 100)

			f.CreateIngress(ns, name, "/a", "svc-default", 80)

			f.WaitNginxConfigStr("listen.*80")

			rules := f.WaitIngressRule(name, ns, 1)
			rule := rules[0]
			ruleName := rule.Name
			_ = ruleName

			f.WaitPolicy(func(policyRaw string) bool {
				hasRule := PolicyHasRule(policyRaw, 80, ruleName)
				hasPod := PolicyHasBackEnds(policyRaw, ruleName, `[map[address:192.168.1.1 otherclusters:false port:80 weight:50] map[address:192.168.2.2 otherclusters:false port:80 weight:50]]`)
				return hasRule && hasPod
			})
		})

		GIt("should work with none path ingress", func() {
			f.AssertKubectlApply(`
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: ingress-onlyhost
  namespace:  alb-test
spec:
  rules:
  - host: local.ares.acp.ingress.domain
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: ingress-normal
  namespace:  alb-test
spec:
  rules:
  - http:
      paths:
      - backend:
          service:
            name: svc-default
            port:
              number: 80
        path: /xx
        pathType: ImplementationSpecific
`)
			f.Wait(func() (bool, error) {
				rs, err := f.K8sClient.GetAlbClient().CrdV1().Rules("cpaas-system").List(f.GetCtx(), metav1.ListOptions{})
				Logf("rs %v %v", rs.Items, err)
				if err != nil {
					return false, err
				}
				return len(rs.Items) == 1, err
			})
		})

		GIt("should handle defaultbackend correctly", func() {
			f.AssertKubectlApply(`
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: ing-nodefault
  namespace: alb-test
spec:
  rules:
  - http:
      paths:
      - backend:
          service:
            name: svc-default
            port:
              number: 80
        path: /xx
        pathType: ImplementationSpecific
`)
			f.Wait(func() (bool, error) {
				ft, err := f.K8sClient.GetAlbClient().CrdV1().Frontends("cpaas-system").Get(f.GetCtx(), "alb-dev-00080", metav1.GetOptions{})
				Logf("%v %v", ft.Spec.ServiceGroup, err)
				if err != nil {
					return false, err
				}
				return ft.Spec.ServiceGroup == nil, nil
			})

			f.AssertKubectlApply(`
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: ing-hasdefault
  namespace: alb-test
spec:
  defaultBackend:
    service:
      name: svc-default
      port:
        number: 80
`)
			f.Wait(func() (bool, error) {
				ft, err := f.K8sClient.GetAlbClient().CrdV1().Frontends("cpaas-system").Get(f.GetCtx(), "alb-dev-00080", metav1.GetOptions{})
				Logf("shoule be 1 %v %v %v", ft.Spec.Source, ft.Spec.ServiceGroup, err)
				if err != nil {
					return false, err
				}
				if ft.Spec.ServiceGroup != nil && ft.Spec.Source.Name == "ing-hasdefault" {
					return true, nil
				}
				return false, nil
			})
			f.AssertKubectlApply(`
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: ing-hasdefault1
  namespace: alb-test
spec:
  defaultBackend:
    service:
      name: svc-default
      port:
        number: 80
`)
			err := f.K8sClient.GetK8sClient().NetworkingV1().Ingresses("alb-test").Delete(f.GetCtx(), "ing-hasdefault", metav1.DeleteOptions{})
			assert.NoError(ginkgo.GinkgoT(), err)
			f.Wait(func() (bool, error) {
				ft, err := f.K8sClient.GetAlbClient().CrdV1().Frontends("cpaas-system").Get(f.GetCtx(), "alb-dev-00080", metav1.GetOptions{})
				Logf("shoule be empty %v %v %v", ft.Spec.Source, ft.Spec.ServiceGroup, err)
				return ft.Spec.ServiceGroup == nil, nil
			})
		})

		GIt("should work when one of ingress rule invalid", func() {
			f.AssertKubectlApply(`
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: ing-xx
  namespace: alb-test
spec:
  rules:
  - http:
      paths:
      - backend:
          service:
            name: svc-default
            port:
              number: 80
        path: /xx1
        pathType: ImplementationSpecific
      - backend:
          service:
            name: svc-default
            port:
              name: notexist
        path: /xx2
        pathType: ImplementationSpecific
`)
			f.Wait(func() (bool, error) {
				rules, err := f.K8sClient.GetAlbClient().CrdV1().Rules("cpaas-system").List(f.GetCtx(), metav1.ListOptions{})
				if err != nil {
					return false, err
				}
				Logf("rules %v %v", len(rules.Items), rules.Items)
				if len(rules.Items) != 1 {
					return false, nil
				}
				eq := reflect.DeepEqual(rules.Items[0].Spec.DSLX[0].Values[0], []string{"STARTS_WITH", "/xx1"})
				return eq, nil
			})

		})

		GIt("should write status in ingress", func() {
			l.Info("hello")
			f.AssertKubectlApply(`
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: ing-xx
  namespace: alb-test
spec:
  rules:
  - host: svc.test
    http:
      paths:
      - backend:
          service:
            name: svc-default
            port:
              number: 80
        path: /xx1
        pathType: ImplementationSpecific
`)
			ctx := f.GetCtx()
			ns := "alb-test"
			ingname := "ing-xx"
			Wait(func() (bool, error) {
				ing, err := f.GetK8sClient().NetworkingV1().Ingresses(ns).Get(ctx, ingname, metav1.GetOptions{})
				if err != nil {
					return false, err
				}
				l.Info("check ingress status", "ing", PrettyCr(ing))
				return NewIngressAssert(ing).HasLoadBalancerIP("127.0.0.1"), nil
			})
			// change alb address
			alb, err := f.GetAlbClient().CrdV2beta1().ALB2s(albns).Get(ctx, albname, metav1.GetOptions{})
			GinkgoNoErr(err)
			alb.Spec.Address = "alauda.com"
			_, err = f.GetAlbClient().CrdV2beta1().ALB2s(albns).Update(ctx, alb, metav1.UpdateOptions{})
			l.Info("update alb address")
			GinkgoNoErr(err)
			Wait(func() (bool, error) {
				ing, err := f.GetK8sClient().NetworkingV1().Ingresses(ns).Get(ctx, ingname, metav1.GetOptions{})
				if err != nil {
					return false, err
				}
				l.Info("check ingress status", "ing", PrettyCr(ing))
				return NewIngressAssert(ing).HasLoadBalancerHostAndPort("alauda.com", []int32{80}), nil
			})

			Wait(func() (bool, error) {
				rs, err := f.GetAlbClient().CrdV1().Rules("cpaas-system").List(ctx, metav1.ListOptions{})
				if err != nil {
					return false, err
				}
				l.Info("rule len ", "len", len(rs.Items))
				for _, r := range rs.Items {
					l.Info("get-rule", "rule", *r.Spec.Source, "name", r.Name)
				}
				return len(rs.Items) == 1, nil
			})

			// 从没有端口到有端口 ngress status中的port应该发生对应变化
			ing, err := f.GetK8sClient().NetworkingV1().Ingresses(ns).Get(ctx, ingname, metav1.GetOptions{})
			GinkgoNoErr(err)
			ing.Annotations["alb.networking.cpaas.io/tls"] = "svc.test=cpaas-system/svc.test-xtgdc"
			ing, err = f.GetK8sClient().NetworkingV1().Ingresses(ns).Update(ctx, ing, metav1.UpdateOptions{})

			Wait(func() (bool, error) {
				ing, err := f.GetK8sClient().NetworkingV1().Ingresses(ns).Get(ctx, ingname, metav1.GetOptions{})
				if err != nil {
					return false, err
				}
				l.Info("check ingress status 443", "ing", PrettyCr(ing))
				return NewIngressAssert(ing).HasLoadBalancerHostAndPort("alauda.com", []int32{443}), nil
			})

			Wait(func() (bool, error) {
				rs, err := f.GetAlbClient().CrdV1().Rules("cpaas-system").List(ctx, metav1.ListOptions{})
				if err != nil {
					return false, err
				}
				l.Info("rule len ", "len", len(rs.Items))
				for _, r := range rs.Items {
					l.Info("get-rule", "rule", *r.Spec.Source, "name", r.Name)
				}
				return len(rs.Items) == 1, nil
			})
		})

		GIt("should not recreate rule when ingress annotation/owner change", func() {
			f.AssertKubectlApply(`
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: ing-xx
  namespace: alb-test
spec:
  rules:
  - http:
      paths:
      - backend:
          service:
            name: svc-default
            port:
              number: 80
        path: /xx1
        pathType: ImplementationSpecific
`)
			ctx := f.GetCtx()
			ns := "alb-test"
			ingname := "ing-xx"
			ing, err := f.GetK8sClient().NetworkingV1().Ingresses(ns).Get(ctx, ingname, metav1.GetOptions{})
			GinkgoNoErr(err)
			ruleName := ""
			// wait for rule created, and get rule name
			f.Wait(func() (bool, error) {
				rs, err := FindRuleViaSource(f.GetCtx(), *f.K8sClient, ing.Name)
				if err != nil {
					return false, err
				}
				if len(rs) != 1 {
					return false, fmt.Errorf("find one more rule %v", PrettyJson(rs))
				}
				r := rs[0]
				l.Info("rule", "rule", PrettyCr(r), "r-i-v", r.Annotations, "ing-v", ing.ResourceVersion)
				ruleName = r.Name
				return true, nil
			})

			// change ingress version
			_, err = f.Kubectl.Kubectl("annotate", "ingresses.networking.k8s.io", "-n", "alb-test", "ing-xx", fmt.Sprintf("a=%d", 1), "--overwrite")
			GinkgoNoErr(err)
			ing, err = f.GetK8sClient().NetworkingV1().Ingresses(ns).Get(ctx, ingname, metav1.GetOptions{})
			GinkgoNoErr(err)

			// rule's name should not change
			f.Wait(func() (bool, error) {
				rs, err := FindRuleViaSource(f.GetCtx(), *f.K8sClient, ing.Name)
				if err != nil {
					return false, err
				}
				if len(rs) != 1 {
					return false, fmt.Errorf("find one more rule %v", PrettyJson(rs))
				}
				r := rs[0]
				l.Info("rule", "rule", PrettyCr(r), "r-i-v", r.Annotations, "ing-v", ing.ResourceVersion)
				if r.Name != ruleName {
					return false, fmt.Errorf("rule name should not change %v %v", ruleName, r.Name)
				}
				return true, nil
			})

		})
	})
})

func FindRuleViaSource(ctx context.Context, cli K8sClient, ing string) ([]av1.Rule, error) {
	rules, err := cli.GetAlbClient().CrdV1().Rules("cpaas-system").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := []av1.Rule{}
	for _, r := range rules.Items {
		if r.Spec.Source.Name == ing {
			out = append(out, r)
		}
	}
	return out, nil
}
