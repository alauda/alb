package cases

import (
	"context"
	"fmt"
	"time"

	. "alauda.io/alb2/test/e2e/framework"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"

	. "alauda.io/alb2/utils/test_utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("cases", func() {
	var kt *Kubectl
	var kc *K8sClient
	var ctx context.Context
	var log logr.Logger
	BeforeEach(func() {
	})

	AfterEach(func() {
	})

	GIt("ACP-28589 cpu limit 1000m", func() {
		env := StartOperatorEnvOrDieWithOpt(OperatorEnvCfg{RunOpext: false})
		kt = env.Kt
		kc = env.Kc
		ctx = env.Ctx
		log = env.Log
		_ = kc
		_ = ctx
		_ = log
		defer env.Stop()

		alb := `
apiVersion: crd.alauda.io/v2beta1
kind: ALB2
metadata:
    name: alb-1
    namespace: cpaas-system
    labels:
        alb.cpaas.io/managed-by: alb-operator
spec:
    type: "nginx" 
    config:
        loadbalancerName: ares-alb2
        resources:
          limits:
            cpu: "1000m"
            memory: "1G"
          requests:
            cpu: "1000m"
            memory: "1G"
        `
		ns := "cpaas-system"
		name := "alb-1"
		_ = name
		_ = ns
		kt.AssertKubectlApply(alb)
		key := types.NamespacedName{Namespace: ns, Name: name}
		ver := ""
		change := 0
		count := 0
		for {
			count++
			_, err := env.Opext.DeployAlb(key, nil)
			GinkgoNoErr(err)
			time.Sleep(200 * time.Millisecond)
			dep, err := kc.GetK8sClient().AppsV1().Deployments(ns).Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				log.Info("dep", "err", err)
				continue
			}
			if dep.ResourceVersion != ver {
				ver = dep.ResourceVersion
				change++
			}
			log.Info("dep==============", "ver", dep.ResourceVersion)
			if change > 10 {
				assert.FailNow(GinkgoT(), "depl should not change")
			}
			if count > 20 {
				break
			}
		}
	})

	GIt("ACP-29703 cert too slow", func() {
		opt := AlbEnvOpt{
			BootYaml: `
        apiVersion: crd.alauda.io/v2beta1
        kind: ALB2
        metadata:
            name: alb-dev
            namespace: cpaas-system
        spec:
            type: "nginx"
            config:
                projects: ["ALL_ALL"]
`,
			Ns:       "cpaas-system",
			Name:     "alb-dev",
			StartAlb: true,
		}
		ns := "cpaas-system"
		env := NewAlbEnvWithOpt(opt)
		kt = env.Kt
		kc = env.Kc
		ctx = env.Ctx
		log = env.Log
		_ = kc
		_ = ctx
		_ = log

		defer env.Stop()
		te := TlsExt{
			Kc:  kc,
			Ctx: ctx,
		}
		te.CreateTlsSecret("a.com", "a", ns)
		for i := 0; i < 10; i++ {
			kt.AssertKubectlApply(Template(`
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: {{.name}}
  namespace: cpaas-system
spec:
  tls:
  - hosts:
      - {{.host}}
    secretName: {{.secret}}
  rules:
  - host: {{.host}}
    http:
      paths:
      - backend:
          service:
            name: apollo
            port:
              number: 8080
        path: /url-{{.count}}
        pathType: ImplementationSpecific
`, map[string]interface{}{
				"host":   fmt.Sprintf("ing.%d.com", i),
				"name":   fmt.Sprintf("ing-%d", i),
				"count":  fmt.Sprintf("%d", i),
				"secret": "a",
			}))
		}
		EventuallySuccess(func(g Gomega) {
			rules, err := kc.GetAlbClient().CrdV1().Rules(ns).List(ctx, metav1.ListOptions{})
			g.Expect(err).NotTo(HaveOccurred())
			log.Info("rules", "len", len(rules.Items))
			g.Expect(len(rules.Items)).Should(Equal(10))
		}, log)
	})
})
