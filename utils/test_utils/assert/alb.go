package assert

import (
	"context"

	a2t "alauda.io/alb2/pkg/apis/alauda/v2beta1"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "alauda.io/alb2/utils/test_utils"
)

func WaitAlbReady(cli *K8sClient, log logr.Logger, ctx context.Context, ns string, name string) *a2t.ALB2 {
	return WaitAlbState(cli, log, ctx, ns, name, func(alb *a2t.ALB2) (bool, error) {
		return alb.Status.State == a2t.ALB2StateRunning, nil
	})
}

func WaitAlbState(cli *K8sClient, log logr.Logger, ctx context.Context, ns string, name string, check func(alb *a2t.ALB2) (bool, error)) *a2t.ALB2 {
	var globalAlb *a2t.ALB2
	Wait(func() (bool, error) {
		alb, err := cli.GetAlbClient().CrdV2beta1().ALB2s(ns).Get(context.Background(), name, metav1.GetOptions{})
		log.Info("try get alb ", "ns", ns, "name", name)
		if errors.IsNotFound(err) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		ok, err := check(alb)
		if err == nil {
			globalAlb = alb
			return ok, nil
		}
		return ok, err
	})
	return globalAlb
}
