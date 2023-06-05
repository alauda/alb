package assert

import (
	"context"
	"time"

	. "alauda.io/alb2/utils/test_utils"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type DeploymentAssert struct {
	kc  *K8sClient
	log logr.Logger
}

func NewDeploymentAssert(kc *K8sClient, log logr.Logger) *DeploymentAssert {
	return &DeploymentAssert{
		kc:  kc,
		log: log,
	}
}

func (d *DeploymentAssert) WaitReady(ctx context.Context, name string, ns string) {
	cli := d.kc.GetK8sClient().AppsV1().Deployments(ns)
	for {
		time.Sleep(time.Second * 1)
		depl, err := cli.Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			d.log.Error(err, "get deployment", "name", name, "ns", ns)
			continue
		}

		if depl.Status.AvailableReplicas != *depl.Spec.Replicas {
			d.log.Info("deployment not ready", "depl", PrettyCr(depl))
			continue
		}
		return
	}
}
