package ingress

import (
	"context"
	"fmt"
	"path"
	"runtime"

	. "alauda.io/alb2/test/e2e/framework"
	. "alauda.io/alb2/utils/test_utils"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func cfd() string {
	_, filename, _, _ := runtime.Caller(0)
	return path.Dir(filename)
}

var _ = Describe("Ingress", func() {
	var env *Env
	var f *IngF
	var l logr.Logger
	var albns string
	var ctx context.Context
	var kt *Kubectl
	var kc *K8sClient
	BeforeEach(func() {
		// "DEFAULT-SSL-CERTIFICATE":   "cpaas-system/default",
		// "DEFAULT-SSL-STRATEGY":      "Always",
		opt := AlbEnvOpt{
			BootYaml: `
        apiVersion: crd.alauda.io/v2beta1
        kind: ALB2
        metadata:
            name: alb-dev
            namespace: cpaas-system
            labels:
                alb.cpaas.io/managed-by: alb-operator
        spec:
            address: "127.0.0.1"
            type: "nginx"
            config:
                networkMode: host
                defaultSSLStrategy: Always
                defaultSSLCert: cpaas-system/default
                projects: ["ALL_ALL"]
`,
			Ns:       "cpaas-system",
			Name:     "alb-dev",
			StartAlb: true,
		}
		env = NewAlbEnvWithOpt(opt)
		ctx = env.Ctx
		l = env.Log
		kt = env.Kt
		_ = kt
		kc = env.K8sClient
		_ = kc
		f = &IngF{
			Kubectl:        env.Kt,
			K8sClient:      env.K8sClient,
			ProductNs:      env.ProductNs,
			AlbWaitFileExt: env.AlbWaitFileExt,
			IngressExt:     env.IngressExt,
			ctx:            ctx,
		}
		albns = env.Opt.Ns
		f.InitProductNs("alb-test", "project1")
		f.InitDefaultSvc("svc-default", []string{"192.168.1.1", "192.168.2.2"})
	})

	AfterEach(func() {
		env.Stop()
	})

	GIt("test ingress startup sync", func() {
		tlog := func(fmts string, arg ...interface{}) {
			l.Info(fmt.Sprintf(fmts, arg))
		}

		expectRuleNum := 81
		f.AssertKubectlApplyFile(path.Join(cfd(), "./all.ingress"))
		f.Wait(func() (bool, error) {
			// 检查rule数量
			rules, err := f.GetAlbClient().CrdV1().Rules(albns).List(f.GetCtx(), v1.ListOptions{})
			if err != nil {
				return false, err
			}
			httpsftid := ""
			for _, r := range rules.Items {
				ft := r.OwnerReferences[0]
				tlog("rule ft id %v %v", string(ft.UID), ft.Name)
				if ft.Name == "alb-dev-00443" && httpsftid == "" {
					httpsftid = string(ft.UID)
				}
				if httpsftid != "" && httpsftid != string(ft.UID) {
					return false, fmt.Errorf("invalid ft id %v | %v", httpsftid, ft.UID)
				}
			}
			tlog("rule len %v", len(rules.Items))
			if len(rules.Items) == expectRuleNum {
				return true, nil
			}
			return false, nil
		})
		tlog("restart alb,it should not recreate rule")
		env.RestartAlb()
		expectWaitCount := 5
		waitCount := 0
		f.Wait(func() (bool, error) {
			// 检查rule数量
			rules, err := f.GetAlbClient().CrdV1().Rules(env.AlbNs).List(f.GetCtx(), v1.ListOptions{})
			if err != nil {
				return false, err
			}
			tlog("rule len %v", len(rules.Items))
			if len(rules.Items) == expectRuleNum {
				waitCount++
			}
			return waitCount == expectWaitCount, nil
		})
	})
})
