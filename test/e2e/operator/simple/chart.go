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
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/types"

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
		values := `
            displayName: "x"
            address: "192.168.134.195"
            projects: ["a","b"]
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
	})

	f.GIt("deploy alb raw mode", func() {
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

		alb := &albv2.ALB2{}
		err := kc.GetClient().Get(ctx, types.NamespacedName{Namespace: "cpaas-system", Name: "ares-alb2"}, alb)
		GinkgoNoErr(err)
		assert.Equal(GinkgoT(), "ares-alb2", *alb.Spec.Config.LoadbalancerName)

		csv, err := kt.Kubectl("get csv -A")
		GinkgoNoErr(err)
		l.Info("csv", "csv", csv)
		assert.Equal(GinkgoT(), strings.Contains(csv, "No resources found"), true)

		depl, err := kt.Kubectl("get deployment -A")
		GinkgoNoErr(err)
		l.Info("depl", "depl", depl)
		assert.Equal(GinkgoT(), strings.Contains(depl, "alb-operator"), true)

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
