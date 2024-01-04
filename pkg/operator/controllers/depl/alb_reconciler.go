/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package depl

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/google/go-cmp/cmp"

	albv2 "alauda.io/alb2/pkg/apis/alauda/v2beta1"
	sharecfg "alauda.io/alb2/pkg/config"
	"alauda.io/alb2/pkg/operator/config"
	. "alauda.io/alb2/pkg/operator/controllers/depl/util"
	"alauda.io/alb2/pkg/operator/toolkit"
	"alauda.io/alb2/utils"
	"github.com/go-logr/logr"
	perr "github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var (
	alb2OperatorFinalizer = "alb2.finalizer"
	alb2Finalizer         = sharecfg.Alb2Finalizer
)

// ALB2Reconciler reconciles a ALB2 object
type ALB2Reconciler struct {
	client.Client
	OperatorCf config.OperatorCfg
	Log        logr.Logger
}

// +kubebuilder:rbac:groups=crd.alauda.io,resources=alaudaloadbalancer2,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=crd.alauda.io,resources=alaudaloadbalancer2/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=crd.alauda.io,resources=alaudaloadbalancer2/finalizers,verbs=update
func (r *ALB2Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	res, err := r.reconcile(ctx, req)
	if err != nil {
		r.Log.Error(err, "reconcile fail")
	}
	return res, err
}

func (r *ALB2Reconciler) reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := r.Log.WithValues("alb", fmt.Sprintf("%s/%s", req.Name, req.Namespace))
	l.Info("reconcile")
	// 处理
	//   alb not found
	//   alb 没有config
	//   config在annotation上
	//   alb 没设置finalizer
	//   alb 删除
	// 这些 case.
	{
		alb := &albv2.ALB2{}
		err := r.Get(ctx, req.NamespacedName, alb)
		if err != nil {
			if errors.IsNotFound(err) {
				l.Info("alb not found ignore")
				return ctrl.Result{}, nil
			}
			l.Error(err, "get alb fail")
			return ctrl.Result{Requeue: true}, nil
		}

		l := l.WithValues("ver", alb.ResourceVersion)
		if alb.Annotations["alb.cpaas.io/ignoreme"] == "true" {
			l.Info("respect ignore me annotation")
			return ctrl.Result{}, nil
		}

		if !r.IsV2Alb(alb) {
			l.Info("api version may be not v2beta1,ignore")
			return ctrl.Result{}, nil
		}

		requeue, err := r.HandleBackupAnnotation(ctx, alb)
		if requeue || err != nil {
			return ctrl.Result{Requeue: true}, err
		}

		// set finalizer if not exist
		if !controllerutil.ContainsFinalizer(alb, alb2OperatorFinalizer) {
			l.Info("finalizer not found update it first")
			controllerutil.AddFinalizer(alb, alb2OperatorFinalizer)
			controllerutil.AddFinalizer(alb, alb2Finalizer)
			if err := r.Update(ctx, alb); err != nil {
				l.Error(err, "unable to add finalizer on alb2", "alb2", alb)
				return ctrl.Result{}, err
			}
			l.Info("finalizer update success", "new-ver", alb.ResourceVersion)
			return ctrl.Result{Requeue: true}, nil
		}

		if !alb.GetDeletionTimestamp().IsZero() {
			l = l.WithName("cleanup")
			deleteTime := alb.GetDeletionTimestamp()
			graceTime := 10
			now := time.Now()
			lastWaitTime := deleteTime.Add(time.Duration(graceTime) * time.Second)
			shouldWait := now.Before(lastWaitTime)
			albcleanuped := !controllerutil.ContainsFinalizer(alb, alb2Finalizer)
			l.Info("deleting alb", "delete", deleteTime, "now", now, "grace", graceTime, "wait", shouldWait, "cleaned", albcleanuped)
			// 如果在给定时间内alb没有把自己的finalizer去掉，那么就直接删除他
			if !albcleanuped && shouldWait {
				l.Info("wait alb cleanup")
				return ctrl.Result{Requeue: true, RequeueAfter: time.Duration(1) * time.Second}, nil
			}
			cur, err := LoadAlbDeploy(ctx, r.Client, r.Log, req.NamespacedName, r.OperatorCf)
			if err != nil {
				return ctrl.Result{}, perr.Wrapf(err, "load alb deploy in delete status fail")
			}
			err = Destroy(ctx, r.Client, r.Log, cur)
			if err != nil {
				l.Error(err, "clear alb2 subResource", "alb2", alb)
				return ctrl.Result{}, perr.Wrapf(err, "destroy alb deploy fail")
			}
			// all clear now.
			controllerutil.RemoveFinalizer(alb, alb2OperatorFinalizer)
			controllerutil.RemoveFinalizer(alb, alb2Finalizer)
			if err := r.Update(ctx, alb); err != nil {
				l.Error(err, "unable to remove finalizer on alb2", "alb2", alb)
				return ctrl.Result{}, perr.Wrapf(err, "remove finalizer,update alb fail")
			}
			// 正常情况下在去掉finalizer之后这个alb就被删除了。重新入队来让他走到not found的状态
			return ctrl.Result{Requeue: true}, nil
		}
	}

	cur, err := LoadAlbDeploy(ctx, r.Client, r.Log, req.NamespacedName, r.OperatorCf)
	if err != nil {
		return ctrl.Result{}, perr.Wrapf(err, "load alb deploy fail")
	}
	albconf, err := config.NewALB2Config(cur.Alb, r.OperatorCf, l)
	if err != nil {
		return ctrl.Result{}, perr.Wrapf(err, "load config fail")
	}
	l.Info("config", "alb conf", utils.PrettyJson(albconf))
	cfg := config.Config{Operator: r.OperatorCf, ALB: *albconf}
	dctl := NewAlbDeployCtl(ctx, r.Client, cfg, r.Log)
	expect, err := dctl.GenExpectAlbDeploy(ctx, cur)
	if err != nil {
		return ctrl.Result{}, perr.Wrapf(err, "gen expect alb deploy fail")
	}
	isReconcile, err := dctl.DoUpdate(ctx, cur, expect)
	if err != nil {
		return ctrl.Result{}, err
	}
	if isReconcile {
		l.Info("update need reconcile")
		return ctrl.Result{Requeue: true}, nil
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ALB2Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Log.Info("set up alb reconcile")
	return ctrl.NewControllerManagedBy(mgr).
		For(&albv2.ALB2{}, builder.WithPredicates(
			predicate.Funcs{
				CreateFunc: func(event event.CreateEvent) bool {
					return true
				},
				DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
					return true
				},
				UpdateFunc: func(e event.UpdateEvent) bool {
					oldObj, oldok := e.ObjectOld.(*albv2.ALB2)
					newObj, newok := e.ObjectNew.(*albv2.ALB2)
					if !oldok || !newok {
						return false
					}

					log := r.Log.WithName("filter").WithValues("name", oldObj.Name, "old-version", oldObj.ResourceVersion, "new-version", newObj.ResourceVersion)
					if !newObj.GetDeletionTimestamp().IsZero() {
						return true
					}
					sameSpec := reflect.DeepEqual(oldObj.Spec, newObj.Spec)
					sameLabel := reflect.DeepEqual(oldObj.Labels, newObj.Labels)
					sameAnnotation := reflect.DeepEqual(oldObj.Annotations, newObj.Annotations)
					sameStatus := SameStatus(oldObj.Status, newObj.Status, log)
					if sameSpec && sameLabel && sameAnnotation && sameStatus {
						return false
					}
					log.Info("alb change", "diff", cmp.Diff(oldObj, newObj))
					return true
				},
				GenericFunc: func(genericEvent event.GenericEvent) bool {
					return true
				},
			},
		)).
		// 当alb的svc变化时(分配了ip)，应该去更新alb的状态
		Watches(&source.Kind{Type: &corev1.Service{}}, handler.EnqueueRequestsFromMapFunc(r.ObjToALbKey),
			builder.WithPredicates(
				predicate.And(
					r.ignoreNonAlbObj(),
					predicate.Funcs{
						CreateFunc: func(event event.CreateEvent) bool {
							return false
						},
						DeleteFunc: func(e event.DeleteEvent) bool {
							r.Log.Info("find svc delete", "svc", toolkit.PrettyCr(e.Object))
							return true
						},
						UpdateFunc: func(e event.UpdateEvent) bool {
							r.Log.Info("find svc update", "name", e.ObjectNew.GetName(), "diff", cmp.Diff(e.ObjectOld, e.ObjectNew))
							return true
						},
						GenericFunc: func(genericEvent event.GenericEvent) bool {
							return false
						},
					},
				),
			),
		).
		// 当alb的deployment变化时(replicas变化)，应该去更新alb的状态
		Watches(&source.Kind{Type: &appsv1.Deployment{}}, handler.EnqueueRequestsFromMapFunc(r.ObjToALbKey),
			builder.WithPredicates(
				predicate.And(
					r.ignoreNonAlbObj(),
					predicate.Funcs{
						CreateFunc: func(event event.CreateEvent) bool {
							// deployment可能就是这里创建的，不关心
							return false
						},
						DeleteFunc: func(e event.DeleteEvent) bool {
							r.Log.Info("find deployment delete", "depl", toolkit.ShowMeta(e.Object))
							return true
						},
						UpdateFunc: func(e event.UpdateEvent) bool {
							odepl, ok := e.ObjectOld.(*appsv1.Deployment)
							if !ok {
								return false
							}
							edepl, ok := e.ObjectNew.(*appsv1.Deployment)
							if !ok {
								return false
							}
							ns, name, version, err := GetAlbKeyFromObject(edepl)
							if err != nil {
								return false
							}
							key := types.NamespacedName{Namespace: ns, Name: name}
							r.Log.Info("find deployment update", "key", key, "version", version, "depl", toolkit.ShowMeta(e.ObjectNew), "old-ver", odepl.ResourceVersion, "new-ver", edepl.ResourceVersion)
							return true
						},
						GenericFunc: func(genericEvent event.GenericEvent) bool {
							return false
						},
					}),
			),
		).
		Complete(r)
}

func (r *ALB2Reconciler) ignoreNonAlbObj() predicate.Funcs {
	return predicate.NewPredicateFuncs(func(object client.Object) bool {
		_, _, version, err := GetAlbKeyFromObject(object)
		if err != nil {
			return false
		}
		if version != r.OperatorCf.Version {
			return false
		}
		return true
	})
}

func (r *ALB2Reconciler) ObjToALbKey(obj client.Object) []reconcile.Request {
	empty := []reconcile.Request{}
	// not alb-operator managed deployment, ignore.
	ns, name, version, err := GetAlbKeyFromObject(obj)
	if err != nil {
		return []ctrl.Request{}
	}
	if version != r.OperatorCf.Version {
		r.Log.Info("version not same,ignore", "depl", toolkit.ShowMeta(obj), "alb2", name, "ver", version, "version", r.OperatorCf.Version)
		return empty
	}
	return []reconcile.Request{
		{
			NamespacedName: types.NamespacedName{Namespace: ns, Name: name},
		},
	}
}
