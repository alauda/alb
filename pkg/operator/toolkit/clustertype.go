package toolkit

import (
	"context"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ClusterTypeIsCCE(cli client.Client, ctx context.Context) (bool, error) {
	cm := corev1.ConfigMap{}
	err := cli.Get(ctx, client.ObjectKey{Namespace: "kube-public", Name: "global-info"}, &cm)
	if err != nil {
		return false, err
	}
	return cm.Data["clusterType"] == "HuaweiCloudCCE", nil
}
