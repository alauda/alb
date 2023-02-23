package patch

import (
	"context"

	cfg "alauda.io/alb2/pkg/operator/config"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
)

func FindConfigmapPatch(ctx context.Context, cli client.Client, conf *cfg.ALB2Config, operator cfg.OperatorCfg) (hasPatch bool, configMap *corev1.ConfigMap, err error) {
	need := false
	name, ns := "", ""
	for _, p := range conf.Overwrite.Configmap {
		if p.Target == "" || p.Target == operator.Version {
			need = true
			ns, name, err = cache.SplitMetaNamespaceKey(p.Name)
			if err != nil {
				return false, nil, err
			}
		}
	}
	if !need {
		return false, nil, nil
	}
	cfg := &corev1.ConfigMap{}
	err = cli.Get(ctx, types.NamespacedName{Namespace: ns, Name: name}, cfg)
	if err != nil {
		return false, nil, err
	}
	return true, cfg, nil
}
