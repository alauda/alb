package cases

import (
	"context"
	"time"

	. "alauda.io/alb2/test/e2e/framework"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo"
	"github.com/stretchr/testify/assert"

	. "alauda.io/alb2/utils/test_utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("cases", func() {
	var env *OperatorEnv
	var kt *Kubectl
	var kc *K8sClient
	var ctx context.Context
	var log logr.Logger
	BeforeEach(func() {
		env = StartOperatorEnvOrDieWithOpt(OperatorEnvCfg{RunOpext: false})
		kt = env.Kt
		kc = env.Kc
		ctx = env.Ctx
		log = env.Log
		_ = kc
		_ = ctx
		_ = log
	})

	AfterEach(func() {
		env.Stop()
	})

	GIt("ACP-28589", func() {
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
			count = count + 1
			_, err := env.Opext.DeployAlb(key, nil)
			GinkgoNoErr(err)
			time.Sleep(time.Duration(200 * time.Millisecond))
			dep, err := kc.GetK8sClient().AppsV1().Deployments(ns).Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				log.Info("dep", "err", err)
				continue
			}
			if dep.ResourceVersion != ver {
				ver = dep.ResourceVersion
				change = change + 1
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
})
