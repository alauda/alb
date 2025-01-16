package perf

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"

	pprof "runtime/pprof"

	"alauda.io/alb2/config"
	"alauda.io/alb2/driver"
	albv1 "alauda.io/alb2/pkg/apis/alauda/v1"
	at "alauda.io/alb2/pkg/controller/ext/auth/types"
	pm "alauda.io/alb2/pkg/utils/metrics"
	ptu "alauda.io/alb2/pkg/utils/test_utils"
	"alauda.io/alb2/utils"
	"alauda.io/alb2/utils/log"
	. "alauda.io/alb2/utils/test_utils"
	k8sv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/rest"
)

// alb 处理规则的速度 5k条规则 87ms alb-perf-go-policy-gen

var _ = Describe("rule perf", func() {
	base := InitBase()
	var env *EnvtestExt
	var kt *Kubectl
	var kc *K8sClient
	var ctx context.Context
	var l logr.Logger
	var ctx_cancel context.CancelFunc
	BeforeEach(func() {
		l = log.L()
		env = NewEnvtestExt(InitBase(), l)
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

	It("should ok when has 5k rule", func() {
		if os.Getenv("RULE_PERF") == "" {
			return
		}
		init_k8s(env.GetRestCfg())
		mock := config.DefaultMock()
		drv, err := driver.NewDriver(driver.DrvOpt{Ctx: ctx, Cf: env.GetRestCfg(), Opt: driver.Cfg2opt(mock)})
		GinkgoNoErr(err)
		s := time.Now()
		ctx := ptu.PolicyGetCtx{
			Ctx: ctx, Name: "alb-dev", Ns: "cpaas-system", Drv: drv, L: l,
			Cfg: mock,
		}
		cli := ptu.NewXCli(ctx)

		f, err := os.Create("rule-perf-cpu")
		if err != nil {
			GinkgoNoErr(err)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			GinkgoNoErr(err)
		}
		defer pprof.StopCPUProfile()
		count := 500
		for i := 1; i <= count; i++ {
			l.Info("perf policy", "i", i, "a", count)
			_, _, err := cli.GetPolicyAndNgx(ctx)
			GinkgoNoErr(err)
		}
		e := time.Now()
		// l.Info("xx", "p", utils.PrettyJson(policy.SharedConfig))
		l.Info("xx", "t", pm.Read())
		l.Info("xx", "all", e.UnixMilli()-s.UnixMilli(), "avg", (e.UnixMilli()-s.UnixMilli())/int64(count))
	})
})

func init_svc_and_ep(ns string, name string, port int, ip string, kt *K8sClient) {
	svc := k8sv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: k8sv1.ServiceSpec{
			Type: k8sv1.ServiceTypeClusterIP,
			Ports: []k8sv1.ServicePort{
				{Port: int32(port), TargetPort: intstr.FromInt(port), Protocol: k8sv1.ProtocolTCP},
			},
		},
	}
	ep := k8sv1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Subsets: []k8sv1.EndpointSubset{
			{
				Ports: []k8sv1.EndpointPort{
					{
						Port:     int32(port),
						Protocol: k8sv1.ProtocolTCP,
					},
				},
				Addresses: []k8sv1.EndpointAddress{
					{
						IP:       ip,
						Hostname: "s-1-ep-2",
					},
				},
			},
		},
	}
	ctx := context.Background()
	kt.GetK8sClient().CoreV1().Services(ns).Create(ctx, &svc, metav1.CreateOptions{})
	kt.GetK8sClient().CoreV1().Endpoints(ns).Create(ctx, &ep, metav1.CreateOptions{})
}

func gen_rule(alb string, ft string, count int, kc *K8sClient) error {
	init_svc_and_ep("cpaas-system", "demo", 80, "192.168.0.1", kc)
	ctx := context.Background()
	for i := 0; i < count; i++ {
		r := albv1.Rule{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("rule-%v", i),
				Namespace: "cpaas-system",
				Labels: map[string]string{
					"alb2.cpaas.io/name":     alb,
					"alb2.cpaas.io/frontend": ft,
				},
			},
			Spec: albv1.RuleSpec{
				Priority: 4,
				DSLX: albv1.DSLX{
					{
						Values: [][]string{{utils.OP_EQ, fmt.Sprintf("/rule-%v", i)}},
						Type:   utils.KEY_URL,
					},
				},
				Config: &albv1.RuleConfigInCr{
					Auth: &at.AuthCr{
						Forward: &at.ForwardAuthInCr{
							Url: "http://a.com",
						},
					},
				},

				ServiceGroup: &albv1.ServiceGroup{
					Services: []albv1.Service{
						{
							Name:      "demo",
							Namespace: "cpaas-system",
							Port:      80,
							Weight:    100,
						},
					},
				},
			},
		}
		_, err := kc.GetAlbClient().CrdV1().Rules("cpaas-system").Create(ctx, &r, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

func init_k8s(cfg *rest.Config) {
	kt := NewKubectl("", cfg, log.L())
	kc := NewK8sClient(context.Background(), cfg)
	kt.AssertKubectlApply(`
apiVersion: crd.alauda.io/v2beta1
kind: ALB2
metadata:
    name: alb-dev
    namespace: cpaas-system
spec:
    address: "127.0.0.1"
    type: "nginx"
    config:
       replicas: 1
---
apiVersion: crd.alauda.io/v1
kind: Frontend
metadata:
  labels:
    alb2.cpaas.io/name: alb-dev
  name: alb-dev-00080
  namespace: cpaas-system
spec:
  backendProtocol: ""
  certificate_name: ""
  port: 80
  protocol: http
`)
	gen_rule("alb-dev", "alb-dev-00080", 5000, kc)
}
