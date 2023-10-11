package checklist

import (
	"fmt"
	_ "fmt"
	"net/url"
	"os"
	"strings"

	. "alauda.io/alb2/test/e2e/framework"
	alog "alauda.io/alb2/utils/log"
	. "alauda.io/alb2/utils/test_utils"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo"
)

func getcwd() string {
	path, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return path
}

func MockCluster(global *EnvtestExt, cluster *EnvtestExt, l logr.Logger) error {
	name := cluster.Name
	globalkc := NewKubectl(global.Base, global.GetRestCfg(), l)
	clusterkc := NewKubectl(cluster.Base, cluster.GetRestCfg(), l)
	l.Info("mock cluster")
	clusterkc.AssertKubectlApply(`
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: debug
rules:
- apiGroups: ["*"]
  resources: ["*"]
  verbs: ['*']
`)
	clusterkc.AssertKubectlApply(`
apiVersion: v1
kind: ServiceAccount
metadata:
  name: debug
`)
	clusterkc.AssertKubectl("create", "clusterrolebinding", "debug", "--clusterrole=debug", "--serviceaccount=default:debug")
	token := clusterkc.AssertKubectl("create", "token", "debug")

	parseHostAndPort := func(host string) (string, string, error) {
		// implementation of parseHostAndPort function
		// https://127.0.0.1:36449/ to 127.0.0.1:36449
		url, err := url.Parse(host)
		if err != nil {
			return "", "", err
		}
		return url.Hostname(), url.Port(), nil
	}

	host, port, err := parseHostAndPort(cluster.GetRestCfg().Host)
	if err != nil {
		return err
	}
	l.Info("debug", "token", token, "host", host, "port", port)
	globalkc.AssertKubectlApply(
		fmt.Sprintf(`
apiVersion: platform.tkestack.io/v1
kind: Cluster
metadata:
  name: %s
spec:
  state: Running
  clusterCredentialRef:
    name: %s
status:
  addresses:
    - host: %s
      port: %s
`, name, name, host, port))
	globalkc.AssertKubectlApply(
		fmt.Sprintf(`
apiVersion: platform.tkestack.io/v1
kind: ClusterCredential
metadata:
  name: %s
spec:
  token: %s
`, name, token))

	return nil
}

var _ = Describe("checklist for alb", func() {
	type ctx struct {
		Global  *EnvtestExt
		Product *EnvtestExt
		Log     logr.Logger
	}
	type cfg struct {
		cringlobal  []string
		crinproduct []string
	}
	test := func(cfg cfg, t func(c ctx)) {
		base := InitBase()
		l := alog.InitKlogV2(alog.LogCfg{ToFile: base + "/chlist.log"})

		global := BaseWithDir(base, "global")
		genv := NewEnvtestExt(global, l).WithName("global").Crds([]string{GetAlbBase() + "/scripts/yaml/crds/extra/mock"})
		genv.AssertStart()
		genvkc := genv.Kubectl()
		defer genv.Stop()

		p1 := BaseWithDir(base, "p1")
		p1env := NewEnvtestExt(p1, l).WithName("p1").Crds([]string{GetAlbBase() + "/scripts/yaml/crds/extra/mock"})
		p1env.AssertStart()
		p1envkc := p1env.Kubectl()
		defer p1env.Stop()

		err := MockCluster(genv, p1env, l)
		GinkgoAssert(err, "mock cluster fail")
		l.Info("mock cluter")

		for _, cr := range cfg.cringlobal {
			genvkc.AssertKubectlApply(cr)
		}
		for _, cr := range cfg.crinproduct {
			p1envkc.AssertKubectlApply(cr)
		}
		t(ctx{Global: genv, Product: p1env, Log: l})
	}

	GIt("should check alb project", func() {
		// 本质上是检查hr上面的alb的项目和alb的label是否一致
		// 防止出现用户手动更改了alb的label，但是hr没改，导致不一致，导致升级的时候项目丢失的问题
		cringlobal := []string{
			`
apiVersion: app.alauda.io/v1
kind: HelmRequest
metadata:
    name: p1-test
    namespace: cpaas-system
spec:
  chart: stable/alauda-alb2
  clusterName: p1
  namespace: cpaas-system
  values:
    address: 192.168.0.201
    projects:
      - ALL_ALL
    replicas: 1
`,
		}
		crinproduct := []string{
			`
apiVersion: crd.alauda.io/v1
kind: ALB2
metadata:
  labels:
    project.cpaas.io/cpaas-system: "true"
  name: test
  namespace: cpaas-system
spec:
  address: 127.0.0.1
  type: nginx
`}
		test(cfg{cringlobal: cringlobal, crinproduct: crinproduct}, func(c ctx) {
			out, err := NewCmd().
				Env(map[string]string{"KUBECONFIG": c.Global.GetKubeCfgPath()}).
				Cwd(GetAlbBase()+"/migrate/checklist").
				Call("bash", "run.sh", "check_alb_project", "v3.10.1", "v3.12.1")
			GinkgoAssert(err, "")
			GinkgoAssertTrue(strings.Contains(out, "p1 alb test  hr与alb资源上的项目不一致, hr资源上的为: ALL_ALL, alb资源上的为: cpaas-system, 请检查"), "")

			c.Product.Kubectl().AssertKubectlApply(`
apiVersion: crd.alauda.io/v1
kind: ALB2
metadata:
  labels:
    project.cpaas.io/ALL_ALL: "true"
  name: test
  namespace: cpaas-system
spec:
  address: 127.0.0.1
  type: nginx
`)
			out, err = NewCmd().
				Env(map[string]string{"KUBECONFIG": c.Global.GetKubeCfgPath()}).
				Cwd(GetAlbBase()+"/migrate/checklist").
				Call("bash", "run.sh", "check_alb_project", "v3.10.1", "v3.12.1")
			GinkgoAssert(err, "")
			GinkgoAssertTrue(strings.Contains(out, "p1 alb test  hr与alb资源上的项目一致, hr资源上的为: ALL_ALL, alb资源上的为: ALL_ALL"), "")
		})
	})

	GIt("should check alb ingress http port", func() {
		// 本质上是检查默认alb的ingress http port在关闭的情况下，是否还有旧的http port 在
		// 在3.10之后才会有这个问题
		// 本质上要检查的是apprelease和ft
		cringlobal := []string{
			`
apiVersion: operator.alauda.io/v1alpha1
kind: AppRelease
metadata:
  name: alauda-alb2
  namespace: cpaas-system
status:
   charts:
     acp/chart-alauda-alb2:
       installedRevision: ""
       phase: ""
       releaseName: ""
       revision: ""
       values:
         ingressHTTPPort: 0
         ingressHTTPSPort: 443
         loadbalancerName: global-alb2
         metricsPort: 11782
---
apiVersion: crd.alauda.io/v1
kind: Frontend
metadata:
  name: global-alb2-00080
  namespace: cpaas-system
spec:
  backendProtocol: ""
  certificate_name: ""
  port: 80
  protocol: http
`}
		crinproduct := []string{
			`
apiVersion: operator.alauda.io/v1alpha1
kind: AppRelease
metadata:
  name: alauda-alb2
  namespace: cpaas-system
status:
   charts:
     acp/chart-alauda-alb2:
       installedRevision: ""
       phase: ""
       releaseName: ""
       revision: ""
       values:
        ingressHTTPPort: 0
        ingressHTTPSPort: 11781
        loadbalancerName: cpaas-system
        metricsPort: 11782
---
apiVersion: crd.alauda.io/v1
kind: Frontend
metadata:
  name: cpaas-system-11780
  namespace: cpaas-system
spec:
  backendProtocol: ""
  certificate_name: ""
  port: 11780
  protocol: http
`}

		test(cfg{cringlobal: cringlobal, crinproduct: crinproduct}, func(c ctx) {

			out, err := NewCmd().
				Env(map[string]string{"KUBECONFIG": c.Global.GetKubeCfgPath()}).
				Cwd(GetAlbBase()+"/migrate/checklist").
				Call("bash", "run.sh", "check_alb_ingress_httpport", "v3.1.1", "v3.2.1")
			GinkgoAssert(err, "")
			GinkgoAssertTrue(strings.Contains(out, " ====v3.2.1 小于v3.10 无需检查===="), "")

			out, err = NewCmd().
				Env(map[string]string{"KUBECONFIG": c.Global.GetKubeCfgPath()}).
				Cwd(GetAlbBase()+"/migrate/checklist").
				Call("bash", "run.sh", "check_alb_ingress_httpport", "v3.10.1", "v3.12.1")
			GinkgoAssert(err, "")
			GinkgoAssertTrue(strings.Contains(out, "global的alb global-alb2 的 ingresshttpport 为0 但仍有ft? 请检查"), "")
			GinkgoAssertTrue(strings.Contains(out, "'p1'的alb cpaas-system 的 ingresshttpport 为0 但仍有ft"), "")

			c.Global.Kubectl().AssertKubectl("delete ft -n cpaas-system global-alb2-00080")
			c.Product.Kubectl().AssertKubectl("delete ft -n cpaas-system cpaas-system-11780")

			out, err = NewCmd().
				Env(map[string]string{"KUBECONFIG": c.Global.GetKubeCfgPath()}).
				Cwd(GetAlbBase()+"/migrate/checklist").
				Call("bash", "run.sh", "check_alb_ingress_httpport", "v3.10.1", "v3.12.1")
			GinkgoAssert(err, "")
			GinkgoAssertTrue(!strings.Contains(out, "global的alb global-alb2 的 ingresshttpport 为0 但仍有ft? 请检查"), "")
			GinkgoAssertTrue(!strings.Contains(out, "'p1'的alb cpaas-system 的 ingresshttpport 为0 但仍有ft"), "")
		})
	})

	GIt("should check alb resources", func() {
		// 本质上是检查默认alb的ingress http port在关闭的情况下，是否还有旧的http port 在
		// 在3.10之后才会有这个问题
		// 本质上要检查的是apprelease和ft
		cringlobal := []string{
			`
apiVersion: operator.alauda.io/v1alpha1
kind: AppRelease
metadata:
  name: alauda-alb2
  namespace: cpaas-system
status:
   charts:
     acp/chart-alauda-alb2:
       installedRevision: ""
       phase: ""
       releaseName: ""
       revision: ""
       values:
        loadbalancerName: global-alb2
        resources:
           limit:
             memory: 100Mi
---
apiVersion: app.alauda.io/v1
kind: HelmRequest
metadata:
  name: p1-test1
  namespace: cpaas-system
spec:
  chart: stable/alauda-alb2
  clusterName: p1
  namespace: cpaas-system
  values:
    address: 192.168.0.201
    projects:
      - ALL_ALL
    replicas: 1
---
apiVersion: app.alauda.io/v1
kind: HelmRequest
metadata:
  name: p1-test2
  namespace: cpaas-system
spec:
  chart: stable/add
---
apiVersion: app.alauda.io/v1
kind: HelmRequest
metadata:
  name: p1-test3
  namespace: cpaas-system
spec:
  chart: stable/alauda-alb2
  clusterName: p1
  namespace: cpaas-system
  values:
    address: 192.168.0.201
    projects:
      - ALL_ALL
    replicas: 1
    resources:
      limits:
        cpu: 2000m
        memory: 256Mi
---
apiVersion: app.alauda.io/v1
kind: HelmRequest
metadata:
  name: p1-test-2
  namespace: cpaas-system
spec:
  chart: stable/alauda-alb2
  clusterName: p1
  namespace: cpaas-system
  values:
    address: 192.168.0.201
    projects:
      - ALL_ALL
    replicas: 1
    resources:
      limits:
        cpu: 2
        memory: 2Gi
`}
		crinproduct := []string{
			`
apiVersion: operator.alauda.io/v1alpha1
kind: AppRelease
metadata:
  name: alauda-alb2
  namespace: cpaas-system
  status:
  charts:
    acp/chart-alauda-alb2:
     values:
       loadbalancerName: cpaas-system
`}

		test(cfg{cringlobal: cringlobal, crinproduct: crinproduct}, func(c ctx) {
			out, err := NewCmd().
				Env(map[string]string{"KUBECONFIG": c.Global.GetKubeCfgPath()}).
				Cwd(GetAlbBase()+"/migrate/checklist").
				Call("bash", "run.sh", "check_alb_resource", "v3.1.1", "v3.2.1")
			GinkgoAssert(err, "")
			GinkgoAssertTrue(strings.Contains(out, "目标升级版本是: v3.2.1, 无需检查alb的resource"), "")

			out, err = NewCmd().Env(map[string]string{"KUBECONFIG": c.Global.GetKubeCfgPath()}).Cwd(GetAlbBase()+"/migrate/checklist").
				Call("bash", "run.sh", "check_alb_resource", "v3.10.1", "v3.12.1")
			GinkgoAssert(err, "")
			GinkgoAssertTrue(strings.Contains(out, "p1 集群的用户alb p1-test1 的hr中没有设置cpulimit 请检查"), "")
			GinkgoAssertTrue(strings.Contains(out, "p1 集群的用户alb p1-test3 的hr中cpulimit格式不正确 请检查"), "")
			_ = out
		})
	})

})
