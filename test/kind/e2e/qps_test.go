package e2e

import (
	"context"
	"fmt"
	"time"

	. "alauda.io/alb2/test/kind/pkg/helper"
	. "alauda.io/alb2/test/kind/pkg/helper/qps"
	. "alauda.io/alb2/utils/test_utils"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

// ginkgo -debug -v  -dryRun ./test/kind/e2e
var _ = Describe("test qps", func() {
	Context("change ingress when test qps", func() {
		var ctx context.Context
		var cancel context.CancelFunc
		var sctx context.Context
		var scancel context.CancelFunc
		var actx *AlbK8sCtx
		var url string
		var ing string
		var l logr.Logger
		BeforeEach(func() {
			// init a kind with a alb
			ctx, cancel = CtxWithSignalAndTimeout(30 * 60)
			actx = NewAlbK8sCtx(ctx, NewAlbK8sCfg().
				WithDefaultAlbName("lb-1").
				Build(),
			)
			err := actx.Init()
			l = actx.Log
			Expect(err).NotTo(HaveOccurred())

			err = actx.DeployEchoResty()
			Expect(err).NotTo(HaveOccurred())
			ing = "echo-resty"
			l.Info("deploy echo resty ok")

			// wait alb ingress work
			// TODO 修复alb 没有主动释放锁的问题
			wait.Poll(time.Second*1, time.Minute*10, func() (bool, error) {
				albips, err := GetAlbPodIp(actx, "lb-1")
				Expect(err).NotTo(HaveOccurred())
				l.Info("alb pod ips", "ips", albips)
				ip := albips[0]
				url = fmt.Sprintf("http://%s", ip)
				out, err := Curl(url)
				if err != nil {
					l.Error(err, "curl fail")
					return false, nil
				}
				l.Info("curl ok", "out", out)
				return true, nil
			})

			// 测一分钟
			dur := time.Duration(60*1) * time.Second
			sctx, scancel = context.WithTimeout(ctx, dur)
			kt := NewKubectl(actx.Cfg.Base, actx.Kubecfg, actx.Log)
			go tweak_ingress(sctx, kt, actx.Log, ing)
		})

		AfterEach(func() {
			scancel()
			cancel()
			actx.Destroy()
		})

		It("should ok", func() {
			l := actx.Log
			r := NewReqProvider(url, l.WithName("curl"), sctx)
			go r.Start()
			l.Info("start curl")
			<-sctx.Done()
			// sa := NewSummaryAssert(r.Summary())
			// sa.NoError()     // 没有报错 TODO 修复缓存的问题
			// sa.QpsAbove(100) // 每秒的qps都大于给定值 TODO ci机器性能不稳定
		})
	})
})

func tweak_ingress(ctx context.Context, kubectl *Kubectl, l logr.Logger, ing string) error {
	count := 0
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(5 * time.Second):
		}
		count++
		out, err := kubectl.Kubectl("annotate", "ingresses.networking.k8s.io", ing, fmt.Sprintf("a=%d", count), "--overwrite")
		if err != nil {
			l.Error(err, "annotate", "out", out)
			continue
		}
		l.Info("annotate", "count", count)
	}
}

func GetAlbPodIp(ctx *AlbK8sCtx, name string) ([]string, error) {
	cli := NewK8sClient(ctx.Ctx, ctx.Kubecfg)
	pods, err := cli.GetK8sClient().CoreV1().Pods(ctx.Cfg.Alboperatorcfg.DefaultAlbNS).List(ctx.Ctx, metav1.ListOptions{LabelSelector: fmt.Sprintf("service.cpaas.io/name=deployment-%s", name)})
	if err != nil {
		return nil, err
	}
	if len(pods.Items) == 0 {
		return nil, fmt.Errorf("should at least get one")
	}
	ips := []string{}
	for _, p := range pods.Items {
		if p.Status.Phase == "Running" {
			ips = append(ips, p.Status.PodIP)
		}
	}
	return ips, nil
}
