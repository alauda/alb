package simple

// test for render alb chart
import (
	"context"
	"strings"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"

	f "alauda.io/alb2/test/e2e/framework"
	"github.com/go-logr/logr"
	_ "github.com/kr/pretty"
	. "github.com/onsi/ginkgo"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/types"

	corev1 "k8s.io/api/core/v1"

	albv2 "alauda.io/alb2/pkg/apis/alauda/v2beta1"
	. "alauda.io/alb2/test/e2e/framework"
	. "alauda.io/alb2/utils/test_utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("chart", func() {
	var ctx context.Context
	var l logr.Logger
	var opext *f.AlbOperatorExt
	var env *OperatorEnv
	var kt *Kubectl
	var kc *K8sClient
	AfterEach(func() {
		_ = opext
	})

	f.GIt("deploy alb csv mode", func() {
		// test point: image secrets
		values := `
            displayName: "x"
            address: "192.168.134.195"
            projects: ["a","b"]
            global:
              labelBaseDomain: cpaas.io
              namespace: cpaas-system
              registry:
                address: build-harbor.alauda.cn
                imagePullSecrets:
                - global-registry-auth
                - xx
            loadbalancerName: ares-alb2
            nodeSelector:
                kubernetes.io/hostname: 192.168.134.195
            gateway:
                enable: true
            replicas: 1
            `
		env = StartOperatorEnvOrDieWithOpt(OperatorEnvCfg{
			RunOpext:      false,
			CsvMode:       true,
			DefaultValues: values,
		})
		defer env.Stop()
		ctx = env.Ctx
		l = env.Log
		opext = env.Opext
		kt = env.Kt
		kc = env.Kc

		alb := &albv2.ALB2{}
		err := kc.GetClient().Get(ctx, types.NamespacedName{Namespace: "cpaas-system", Name: "ares-alb2"}, alb)
		GinkgoNoErr(err)
		assert.Equal(GinkgoT(), "ares-alb2", *alb.Spec.Config.LoadbalancerName)
		assert.Equal(GinkgoT(), "true", alb.Labels["project.cpaas.io/a"])
		assert.Equal(GinkgoT(), "true", alb.Labels["project.cpaas.io/b"])
		l.Info("alb", "alb", alb)

		csv, err := kt.Kubectl("get csv -A")
		GinkgoNoErr(err)
		l.Info("csv", "csv", csv)
		assert.Equal(GinkgoT(), strings.Contains(csv, "alb-operator.v0.1.0"), true)

		deplstr, err := kt.Kubectl("get deployment -A")
		GinkgoNoErr(err)
		l.Info("depl", "depl", deplstr)
		// csv模式的时候,我们不会定义deployment,所以这里应该是空的
		assert.Equal(GinkgoT(), strings.Contains(deplstr, "No resources found"), true)
		l.Info("alb", "annotation", alb.Annotations["alb.cpaas.io/migrate-backup"])
		// 1. sa 上应该设置imagepullsecrets
		ns := "cpaas-system"
		sa, err := kc.GetK8sClient().CoreV1().ServiceAccounts(ns).Get(ctx, "alb-operator", metav1.GetOptions{})
		GinkgoNoErr(err)

		csv, err = kt.Kubectl("get csv -A -o yaml")
		GinkgoNoErr(err)
		l.Info("sa", "sa", PrettyCr(sa), "csv", csv)
		assert.Equal(GinkgoT(), lo.Map(sa.ImagePullSecrets, func(k corev1.LocalObjectReference, _ int) string { return k.Name }), []string{"global-registry-auth", "xx"})
		assert.Equal(GinkgoT(), strings.Contains(csv, "global-registry-auth,xx"), true)

		// https://jira.alauda.cn/browse/ACP-30778
		// operator的nodeselector和默认的alb保持一致
		sel := kt.AssertKubectl("get csv -n cpaas-system alb-operator.v0.1.0 -o=jsonpath='{.spec.install.spec.deployments[0].spec.template.spec.nodeSelector}'")
		assert.Equal(GinkgoT(), sel == "'{\"kubernetes.io/hostname\":\"192.168.134.195\",\"kubernetes.io/os\":\"linux\"}'", true)
		l.Info("csv", "csv", csv, "sel", sel)
	})

	f.GIt("deploy alb deployment mode", func() {
		cfg :=
			`
            operatorDeployMode: "deployment"
            displayName: "x"
            address: "192.168.134.195"
            global:
              labelBaseDomain: cpaas.io
              namespace: cpaas-system
              registry:
                address: build-harbor.alauda.cn
                imagePullSecrets:
                - global-registry-auth
                - xx
            loadbalancerName: ares-alb2
            nodeSelector:
                kubernetes.io/hostname: 192.168.134.195
                alb: "true"
                "1": "true"
                "xtrue": "true"
            gateway:
                enable: true
            replicas: 1
            `
		env = StartOperatorEnvOrDieWithOpt(OperatorEnvCfg{
			RunOpext:      false,
			DefaultValues: cfg,
		})
		defer env.Stop()
		ctx = env.Ctx
		l = env.Log
		opext = env.Opext
		kt = env.Kt
		kc = env.Kc

		alb := &albv2.ALB2{}
		err := kc.GetClient().Get(ctx, types.NamespacedName{Namespace: "cpaas-system", Name: "ares-alb2"}, alb)
		GinkgoNoErr(err)
		assert.Equal(GinkgoT(), "ares-alb2", *alb.Spec.Config.LoadbalancerName)

		csv, err := kt.Kubectl("get csv -A")
		GinkgoNoErr(err)
		l.Info("csv", "csv", csv)
		assert.Equal(GinkgoT(), strings.Contains(csv, "No resources found"), true)

		l.Info("alb", "annotation", alb.Annotations["alb.cpaas.io/migrate-backup"])
		_, err = kc.GetK8sClient().RbacV1().ClusterRoleBindings().Get(ctx, "alb-operator", metav1.GetOptions{})
		GinkgoNoErr(err)
		_, err = kc.GetK8sClient().RbacV1().ClusterRoles().Get(ctx, "alb-operator", metav1.GetOptions{})
		GinkgoNoErr(err)
		_, err = kc.GetK8sClient().CoreV1().ServiceAccounts("cpaas-system").Get(ctx, "alb-operator", metav1.GetOptions{})
		GinkgoNoErr(err)
		dep, err := kc.GetK8sClient().AppsV1().Deployments("cpaas-system").Get(ctx, "alb-operator", metav1.GetOptions{})
		GinkgoNoErr(err)
		assert.Equal(GinkgoT(), dep.Spec.Template.Spec.ServiceAccountName, "alb-operator")

		sa, err := kc.GetK8sClient().CoreV1().ServiceAccounts("cpaas-system").Get(ctx, "alb-operator", metav1.GetOptions{})
		GinkgoNoErr(err)
		l.Info("depl", "sa", PrettyCr(sa))
		assert.Equal(GinkgoT(), lo.Map(sa.ImagePullSecrets, func(k corev1.LocalObjectReference, _ int) string { return k.Name }), []string{"global-registry-auth", "xx"})
		deplyaml, err := kt.Kubectl("get deployment -n cpaas-system alb-operator -o yaml")
		GinkgoNoErr(err)
		l.Info("depl", "yaml", deplyaml)
		assert.Equal(GinkgoT(), strings.Contains(deplyaml, "global-registry-auth,xx"), true)

		// https://jira.alauda.cn/browse/ACP-30778
		// operator的nodeselector和默认的alb保持一致
		assert.Equal(GinkgoT(), dep.Spec.Template.Spec.NodeSelector, map[string]string{
			"kubernetes.io/hostname": "192.168.134.195",
			"kubernetes.io/os":       "linux",
			"alb":                    "true",
			"xtrue":                  "true",
			"1":                      "true",
		})

	})

	f.GIt("deploy operator only", func() {
		cfg :=
			`
            operatorDeployMode: "deployment"
            defaultAlb: false
            displayName: "x"
            address: "192.168.134.195"
            global:
              labelBaseDomain: cpaas.io
              namespace: cpaas-system
              registry:
                address: build-harbor.alauda.cn
            loadbalancerName: ares-alb2
            nodeSelector:
                kubernetes.io/hostname: 192.168.134.195
            gateway:
                enable: true
            replicas: 1
            `
		env = StartOperatorEnvOrDieWithOpt(OperatorEnvCfg{
			RunOpext:      false,
			DefaultValues: cfg,
		})
		defer env.Stop()
		ctx = env.Ctx
		l = env.Log
		opext = env.Opext
		kt = env.Kt
		kc = env.Kc

		depl, err := kt.Kubectl("get deployment -A")
		GinkgoNoErr(err)
		l.Info("depl", "depl", depl)
		assert.Equal(GinkgoT(), strings.Contains(depl, "alb-operator"), true)
		alb := &albv2.ALB2{}
		// 不应该有默认的alb
		err = kc.GetClient().Get(ctx, types.NamespacedName{Namespace: "cpaas-system", Name: "ares-alb2"}, alb)
		assert.True(GinkgoT(), k8serrors.IsNotFound(err))
	})
})
