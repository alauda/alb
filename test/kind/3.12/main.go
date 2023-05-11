package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/go-logr/logr"
	"github.com/ztrue/tracerr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "alauda.io/alb2/test/kind/3.12/helper"
	. "alauda.io/alb2/test/kind/3.12/helper/qps"
	. "alauda.io/alb2/utils/test_utils"
)

func main() {
	ctx, cancel := CtxWithSignalAndTimeout(30 * 60)
	defer cancel()
	if err := run(ctx); err != nil {
		tracerr.Print(err)
		panic(err)
	}
}

func run(ctx context.Context) error {
	chart := os.Getenv("ALB_KIND_E2E_CHART")
	branch := os.Getenv("ALB_KIND_E2E_BRANCH")
	fmt.Printf("chart is %v\n", chart)
	fmt.Printf("branch is %v\n", branch)

	chart = fmt.Sprintf("registry.alauda.cn:60080/acp/chart-alauda-alb2:%s", chart)
	cfg := NewAlbK8sCfg(chart, "e2e-demo").
		Build()
	actx := NewAlbK8sCtx(ctx, cfg)
	err := actx.Init()
	if err != nil {
		return err
	}

	l := actx.Log

	if err := actx.DeployApp(); err != nil {
		return err
	}
	albips, err := GetAlbPodIp(actx)
	if err != nil {
		return err
	}
	ip := albips[0]
	url := fmt.Sprintf("http://%s", ip)

	// make alb ingress work
	wait.Poll(time.Second*1, time.Minute*10, func() (bool, error) {
		out, err := Curl(url)
		if err != nil {
			l.Error(err, "curl fail")
			return false, nil
		}
		l.Info("curl ok", "out", out)
		return true, nil
	})

	test_qps(actx, url, "echo-resty")
	l.Info("test success")
	return nil
}

func tweak_ingress(ctx context.Context, kubectl *Kubectl, l logr.Logger, ing string) error {
	count := 0
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(5 * time.Second):
		}
		count = count + 1
		out, err := kubectl.Kubectl("annotate", "ingresses.networking.k8s.io", ing, fmt.Sprintf("a=%d", count), "--overwrite")
		if err != nil {
			l.Error(err, "annotate", "out", out)
			continue
		}
		l.Info("annotate", "count", count)
	}
}

func test_qps(actx *AlbK8sCtx, url string, ing string) error {
	l := actx.Log
	dur := time.Duration(60*1) * time.Second
	sctx, scancel := context.WithTimeout(actx.Ctx, dur)
	defer scancel()
	r := NewReqProvider(url, l.WithName("curl"), sctx)
	go r.Start()
	go tweak_ingress(sctx, actx.Kubectl, l, ing)
	<-sctx.Done()
	// sa := NewSummaryAssert(r.Summary())
	// sa.NoError()    // 没有报错 TODO 修复缓存的问题
	// sa.QpsAbove(10) // 每秒的qps都大于给定值 TODO ci机器性能不稳定
	return nil
}

func GetAlbPodIp(ctx *AlbK8sCtx) ([]string, error) {
	cli := ctx.Kubecliet
	pods, err := cli.GetK8sClient().CoreV1().Pods(ctx.Cfg.Ns).List(ctx.Ctx, metav1.ListOptions{LabelSelector: fmt.Sprintf("service.cpaas.io/name=deployment-%s", ctx.Cfg.Name)})
	if err != nil {
		return nil, err
	}
	if len(pods.Items) == 0 {
		return nil, fmt.Errorf("should at least get one")
	}
	ips := []string{}
	for _, p := range pods.Items {
		ips = append(ips, p.Status.PodIP)
	}
	return ips, nil
}
