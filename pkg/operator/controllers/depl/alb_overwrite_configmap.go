package depl

import (
	"context"
	"reflect"

	c "alauda.io/alb2/config"
	albv2 "alauda.io/alb2/pkg/apis/alauda/v2beta1"
	"alauda.io/alb2/pkg/operator/config"
	patch "alauda.io/alb2/pkg/operator/controllers/depl/patch"
	"alauda.io/alb2/pkg/operator/toolkit"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

func (r *ALB2Reconciler) updateOverwriteConfigmapIfNeed(ctx context.Context, cfg config.Config) error {
	if len(cfg.ALB.Overwrite.Configmap) == 0 {
		return nil
	}
	has, cm, err := patch.FindConfigmapPatch(ctx, r.Client, &cfg.ALB, r.OperatorCf)
	if !has {
		return nil
	}
	if err != nil {
		return nil
	}

	key := c.NewNames(cfg.ALB.Domain).GetOverwriteConfigmapLabelKey()
	if cm.Labels == nil {
		cm.Labels = map[string]string{}
	}
	if cm.Labels[key] == "true" {
		return nil
	}
	cm.Labels[key] = "true"
	r.Log.Info("patch overwrite configmap label", "key", key)
	return r.Client.Update(ctx, cm)
}

func (r *ALB2Reconciler) setUpWatchConfigmap(b *builder.Builder) {
	// 一个configmap可能被多个alb使用
	// 每次configmap变化时，找使用这个configmap的alb
	//    1 在configmap上加label标识这个configmap是被用来做alb的overwrite的
	//    2 在alb的label上标识这个alb是否被overwrite了
	//  这样在大部分情况下我们能过滤掉那些我们不关心的configmap
	key := c.NewNames(r.OperatorCf.BaseDomain).GetOverwriteConfigmapLabelKey()
	b.Watches(&corev1.ConfigMap{}, handler.EnqueueRequestsFromMapFunc(r.configMapToAlb),
		builder.WithPredicates(
			predicate.And(
				predicate.NewPredicateFuncs(func(object client.Object) bool {
					return object.GetLabels()[key] == "true"
				}),
				predicate.Funcs{
					CreateFunc: func(event event.CreateEvent) bool {
						return true
					},
					DeleteFunc: func(e event.DeleteEvent) bool {
						r.Log.Info("find configmap delete", "cm", toolkit.ShowMeta(e.Object))
						return true
					},
					UpdateFunc: func(e event.UpdateEvent) bool {
						oconf, ok := e.ObjectOld.(*corev1.ConfigMap)
						if !ok {
							return false
						}
						econf, ok := e.ObjectNew.(*corev1.ConfigMap)
						if !ok {
							return false
						}
						return !reflect.DeepEqual(oconf.Data, econf.Data)
					},
					GenericFunc: func(genericEvent event.GenericEvent) bool {
						return false
					},
				}),
		),
	)
}

func (r *ALB2Reconciler) configMapToAlb(ctx context.Context, cm client.Object) []ctrl.Request {
	ret := []ctrl.Request{}
	albList := albv2.ALB2List{}
	key := c.NewNames(r.OperatorCf.BaseDomain).GetOverwriteConfigmapLabelKey()
	err := r.Client.List(ctx, &albList, &client.ListOptions{LabelSelector: labels.SelectorFromSet(map[string]string{key: "true"})})
	if err != nil {
		return nil
	}
	for _, alb := range albList.Items {
		if alb.UseConfigmap(client.ObjectKeyFromObject(cm)) {
			ret = append(ret, ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: alb.Namespace,
					Name:      alb.Name,
				},
			})
		}
	}
	return ret
}
