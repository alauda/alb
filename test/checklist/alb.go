package checklist

import (
	"fmt"
	"net/url"
	"strings"

	. "alauda.io/alb2/test/e2e/framework"
	alog "alauda.io/alb2/utils/log"
	. "alauda.io/alb2/utils/test_utils"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
)

func MockCluster(global *EnvtestExt, cluster *EnvtestExt, l logr.Logger) error {
	name := cluster.Name
	globalkc := NewKubectl(global.Base, global.GetRestCfg(), l)
	clusterkc := NewKubectl(cluster.Base, cluster.GetRestCfg(), l)
	l.Info("mock cluster")
	l.Info("debug ", "ns", clusterkc.AssertKubectl("get ns -A"))
	l.Info("debug ", "ctx", clusterkc.AssertKubectl("config get-contexts"))
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
  namespace: default
`)
	clusterkc.AssertKubectl("create", "clusterrolebinding", "debug", "--clusterrole=debug", "--serviceaccount=default:debug")
	token := clusterkc.AssertKubectl("create token debug --namespace default")

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

func EvalCheck(kconf string, args []string) (string, error) {
	call_args := []string{"run.sh"}
	call_args = append(call_args, args...)
	out, err := NewCmd().
		Env(map[string]string{"KUBECONFIG": kconf}).
		Cwd(GetAlbBase()+"/migrate/checklist").
		Call("bash", call_args...)
	return out, err
}

func EvalCheckForClusters(kconf string, cmd string, prdb string, target string) (string, error) {
	call_args := []string{"test.sh"}
	call_args = append(call_args, "run-in-clusters", cmd)
	out, err := NewCmd().
		Env(map[string]string{
			"KUBECONFIG":     kconf,
			"target_version": target,
			"prdb_version":   prdb,
			"backup_dir":     "./",
		}).
		Cwd(GetAlbBase()+"/migrate/checklist").
		Call("bash", call_args...)
	return out, err
}

func EvalTest(args []string) (string, error) {
	call_args := []string{"test.sh"}
	call_args = append(call_args, args...)
	out, err := NewCmd().
		Cwd(GetAlbBase()+"/migrate/checklist").
		Call("bash", call_args...)
	return out, err
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
	base := InitBase()
	l := alog.InitKlogV2(alog.LogCfg{ToFile: base + "/chlist.log"})
	test := func(cfg cfg, t func(c ctx)) {
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
		l.Info("mock cluster")

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
---
apiVersion: app.alauda.io/v1
kind: HelmRequest
metadata:
    name: p1-test-1
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
    name: p11-test
    namespace: cpaas-system
spec:
  chart: stable/alauda-alb2
  clusterName: p11
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
    project.cpaas.io/ALL_ALL: "true"
  name: test-1
  namespace: cpaas-system
spec:
  address: 127.0.0.1
  type: nginx
---
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
`,
		}
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
`,
		}
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
`,
		}

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

	GIt("should check alb ingress path", func() {
		cringlobal := []string{
			`
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: test-ing
spec:
  rules:
  - http:
      paths:
      - backend:
          service:
            name: echo-resty-b
            port:
              number: 80
        path: /abc.*
        pathType: ImplementationSpecific
      - backend:
          service:
            name: echo-resty
            port:
              number: 80
        path: /
        pathType: ImplementationSpecific
`,
		}
		crinproduct := []string{
			`
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: test-ing
spec:
  rules:
  - http:
      paths:
      - backend:
          service:
            name: echo-resty-b
            port:
              number: 80
        path: /abc.*
        pathType: ImplementationSpecific
      - backend:
          service:
            name: echo-resty
            port:
              number: 80
        path:   /   
        pathType: ImplementationSpecific
`,
		}

		test(cfg{cringlobal: cringlobal, crinproduct: crinproduct}, func(c ctx) {
			out, err := NewCmd().
				Env(map[string]string{"KUBECONFIG": c.Global.GetKubeCfgPath()}).
				Cwd(GetAlbBase()+"/migrate/checklist").
				Call("bash", "run.sh", "check_alb_ingress_path", "v3.6.1", "v3.8.1")
			GinkgoAssert(err, "")
			GinkgoAssertTrue(strings.Contains(out, "集群 global 发现1 个 path: /的ingress 请检查"), "")
			GinkgoAssertTrue(strings.Contains(out, "集群 p1 发现1 个 path: /的ingress 请检查"), "")
			_ = out
		})
	})

	GIt("should check alb resources", func() {
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
  name: p1-test-4
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
        cpu: "2"
        memory: 2Gi
---
apiVersion: app.alauda.io/v1
kind: HelmRequest
metadata:
  name: p1-test-5
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
`,
		}
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
`,
		}

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

	GIt("should check alb hr", func() {
		cringlobal := []string{
			`
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
    loadbalancerName: p1-test1
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
  chart: stable/alauda-alb2
  clusterName: p1
  namespace: cpaas-system
  values:
    address: 192.168.0.201
    loadbalancerName: p1-test2
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
  name: test1
  namespace: cpaas-system
spec:
  address: 127.0.0.1
  type: nginx
`,
		}

		test(cfg{cringlobal: cringlobal, crinproduct: crinproduct}, func(c ctx) {
			out, err := NewCmd().
				Env(map[string]string{"KUBECONFIG": c.Global.GetKubeCfgPath()}).
				Cwd(GetAlbBase()+"/migrate/checklist").
				Call("bash", "run.sh", "check_alb_hr", "v3.6.1", "v3.8.1")
			GinkgoAssert(err, "")
			GinkgoAssertTrue(!strings.Contains(out, "p1 集群的alb p1-test1 hr还在但是alb不在了"), "")
			GinkgoAssertTrue(strings.Contains(out, "p1 集群的alb p1-test2 hr还在但是alb不在了"), "")
			_ = out
		})
	})

	GIt("should check alb rule", func() {
		cringlobal := []string{
			`
apiVersion: crd.alauda.io/v1
kind: Rule
metadata:
  labels:
    alb2.cpaas.io/frontend: alb-demo-00080  # required, indicate the Frontend to which this rule belongs
    alb2.cpaas.io/name: alb-demo            # required, indicate the ALB to which this rule belongs
    alb2.cpaas.io/source-type: ingress
  name: x-60080-1
  namespace: cpaas-system
spec:
  source:
    name: test-ing
    namespace: cpaas-system
    type: ingress
  type: ""
  url: /
  vhost: ""
---
apiVersion: crd.alauda.io/v1
kind: Rule
metadata:
  labels:
    alb2.cpaas.io/source-type: ingress
    alb2.cpaas.io/source-name: b.cpaas-system
  name: x-60080-2
  namespace: cpaas-system
spec:
  type: ""
  url: /
  vhost: ""
---
apiVersion: crd.alauda.io/v1
kind: Rule
metadata:
  labels:
    alb2.cpaas.io/source-type: ingress
    alb2.cpaas.io/source-name: a.cpaas-system
  name: x-60080-3
  namespace: cpaas-system
spec:
  type: ""
  url: /
  vhost: ""
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: a
  namespace: cpaas-system
spec:
  rules:
  - http:
      paths:
      - backend:
          service:
            name: none
            port:
              number: 8080
        path: /$
        pathType: ImplementationSpecific
`,
		}
		crinproduct := []string{}

		test(cfg{cringlobal: cringlobal, crinproduct: crinproduct}, func(c ctx) {
			out, err := NewCmd().
				Env(map[string]string{"KUBECONFIG": c.Global.GetKubeCfgPath()}).
				Cwd(GetAlbBase()+"/migrate/checklist").
				Call("bash", "run.sh", "check_alb_legacy", "v3.6.1", "v3.8.1")
			GinkgoAssert(err, "")
			GinkgoAssertTrue(strings.Contains(out, "集群 global 的rule的ingress test-ing 不存在"), "")
			GinkgoAssertTrue(strings.Contains(out, "集群 global 的rule的ingress b 不存在"), "")
			GinkgoAssertTrue(!strings.Contains(out, "集群 global 的rule的ingress a 不存在"), "")
			_ = out
		})
	})

	GIt("should check alb exist but hr not exist", func() {
		cr_in_global := []string{
			`

`,
		}
		cr_in_product := []string{
			`
apiVersion: crd.alauda.io/v1
kind: ALB2
metadata:
  name: test-b
  namespace: cpaas-system
spec:
  address: 127.0.0.1
  type: nginx
---
apiVersion: crd.alauda.io/v1
kind: ALB2
metadata:
  name: test-a-1
  namespace: cpaas-system
spec:
  address: 127.0.0.1
  type: nginx
`,
		}
		test(cfg{cringlobal: cr_in_global, crinproduct: cr_in_product}, func(c ctx) {
			out, err := EvalCheck(c.Global.GetKubeCfgPath(), []string{"check_alb_hr", "v3.6.1", "v3.8.1"})
			GinkgoNoErr(err)
			GinkgoAssertTrue(strings.Contains(out, "集群的alb p1-test-a hr还在但是alb不在了"), "")
			GinkgoAssertTrue(strings.Contains(out, "集群的alb p1-test-a-1 alb还在但是hr不在了"), "")
			GinkgoAssertTrue(strings.Contains(out, "集群的alb p1-test-b alb还在但是hr不在了"), "")
		})
	})

	GIt("compare ver should ok", func() {
		type Case struct {
			args   []string
			expect string
		}
		cases := []Case{
			{args: []string{"v3.6.1", "eq", "v3.8.1"}, expect: "false"},
			{args: []string{"v3.6.1", "eq", "v3.6.1"}, expect: "true"},

			{args: []string{"v3.6.1", "lt", "v3.8.1"}, expect: "true"},
			{args: []string{"v3.6.1", "lt", "v3.4.1"}, expect: "false"},
			{args: []string{"v3.6.1", "le", "v3.8.1"}, expect: "true"},
			{args: []string{"v3.6.1", "le", "v3.6.1"}, expect: "true"},

			{args: []string{"v3.6.1", "gt", "v3.4.1"}, expect: "true"},
			{args: []string{"v3.6.1", "gt", "v3.8.1"}, expect: "false"},
			{args: []string{"v3.6.1", "ge", "v3.4.1"}, expect: "true"},
			{args: []string{"v3.6.1", "ge", "v3.8.1"}, expect: "false"},
		}
		for _, c := range cases {
			args := []string{"_alb_compare_ver"}
			args = append(args, c.args...)
			out, err := EvalTest(args)
			GinkgoNoErr(err)
			GinkgoAssertTrue(strings.Contains(out, c.expect), "")
		}
	})

	GIt("should notice portproject need re-add to configmap cur (,3.8.2) and target (,3.8.2)", func() {
		cr_in_global := []string{
			`
apiVersion: crd.alauda.io/v1
kind: ALB2
metadata:
  name: test-no-port
  namespace: cpaas-system
spec:
  address: 127.0.0.1
  type: nginx
---
apiVersion: v1
kind: ConfigMap
metadata:
  labels:
    name: test-no-port
    port-info: "true"
  name: test-no-port-port-info
  namespace: cpaas-system
data:
  range: '[]'
`,
		}
		cr_in_product := []string{
			`
apiVersion: crd.alauda.io/v1
kind: ALB2
metadata:
  name: test-without-port-cm
  namespace: cpaas-system
spec:
  address: 127.0.0.1
  type: nginx
---
apiVersion: crd.alauda.io/v1
kind: ALB2
metadata:
  name: test
  namespace: cpaas-system
spec:
  address: 127.0.0.1
  type: nginx
---
apiVersion: v1
kind: ConfigMap
metadata:
  creationTimestamp: "2024-09-24T15:49:17Z"
  labels:
    name: test
    port-info: "true"
  name: test-port-info
  namespace: cpaas-system
data:
  range: '[{"port":"111-2233","projects":["ALL_ALL"]}]'
`,
		}
		test(cfg{cringlobal: cr_in_global, crinproduct: cr_in_product}, func(c ctx) {
			out, err := EvalCheckForClusters(c.Global.GetKubeCfgPath(), "check_alb_port_project", "v3.6.1", "v3.8.1")
			GinkgoNoErr(err)
			_ = out
		})
	})

	GIt("should notice portproject need re-add to hr, target [3.8.2,3.12.0)", func() {
		cr_in_global := []string{
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
    portProjects: '[]'
---
apiVersion: app.alauda.io/v1
kind: HelmRequest
metadata:
    name: p1-test-a
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
    portProjects: '[{"port":"111-2233","projects":["ALL_ALL"]}]'
`,
		}
		cr_in_product := []string{
			`
apiVersion: crd.alauda.io/v1
kind: ALB2
metadata:
  name: test
  namespace: cpaas-system
spec:
  address: 127.0.0.1
  type: nginx
---
apiVersion: v1
kind: ConfigMap
metadata:
  labels:
    name: test
    port-info: "true"
  name: test-port-info
  namespace: cpaas-system
data:
  range: '[{"port":"111-2233","projects":["ALL_ALL"]}]'
---
apiVersion: crd.alauda.io/v1
kind: ALB2
metadata:
  name: test-a
  namespace: cpaas-system
spec:
  address: 127.0.0.1
  type: nginx
---
apiVersion: v1
kind: ConfigMap
metadata:
  labels:
    name: test-a
    port-info: "true"
  name: test-a-port-info
  namespace: cpaas-system
data:
  range: '[{"port":"111-2233","projects":["ALL_ALL"]}]'
`,
		}
		test(cfg{cringlobal: cr_in_global, crinproduct: cr_in_product}, func(c ctx) {
			out, err := EvalCheckForClusters(c.Global.GetKubeCfgPath(), "check_alb_port_project", "v3.8.2", "v3.12.0")
			GinkgoNoErr(err)
			GinkgoAssertTrue(strings.Contains(out, `集群 p1 的 alb test 的端口项目信息 configmap 与 hr不一致 cm: [{"port":"111-2233","projects":["ALL_ALL"]}] hr: []  请更新hr p1-test`), "")
			_ = out
		})
	})

	GIt("should notice mis used cpaas-system project ", func() {
		cr_in_global := []string{}
		cr_in_product := []string{
			`
apiVersion: crd.alauda.io/v1
kind: Rule
metadata:
  labels:
    alb2.cpaas.io/frontend: cpaas-system-11780  # required, indicate the Frontend to which this rule belongs
    alb2.cpaas.io/name: cpaas-system            # required, indicate the ALB to which this rule belongs
  name: x-60080-1
  namespace: cpaas-system
spec:
  source:
    name: test-ing
    namespace: cpaas-system
    type: ingress
  type: ""
  url: /
  vhost: ""
`,
		}
		test(cfg{cringlobal: cr_in_global, crinproduct: cr_in_product}, func(c ctx) {
			out, err := EvalCheckForClusters(c.Global.GetKubeCfgPath(), "check_alb_default_http", "v3.12.2", "v3.18.0")
			GinkgoNoErr(err)
			GinkgoAssertTrue(strings.Contains(out, `发现用户自建规则 |x-60080-1`), "")
			_ = out
		})
	})
})
