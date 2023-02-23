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

package controllers

import (
	"context"
	"fmt"
	"github.com/google/go-cmp/cmp"
	"reflect"
	"strings"

	albv2 "alauda.io/alb2/pkg/apis/alauda/v2beta1"
	"alauda.io/alb2/pkg/operator/config"
	"alauda.io/alb2/pkg/operator/controllers/depl"
	"alauda.io/alb2/pkg/operator/controllers/depl/resources"
	"alauda.io/alb2/pkg/operator/toolkit"

	"github.com/go-logr/logr"
	perr "github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
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

var alb2OperatorFinalizer = "alb2.finalizer"

// ALB2Reconciler reconciles a ALB2 object
type ALB2Reconciler struct {
	client.Client
	Env config.OperatorCfg
	Log logr.Logger
}

// +kubebuilder:rbac:groups=crd.alauda.io,resources=alaudaloadbalancer2,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=crd.alauda.io,resources=alaudaloadbalancer2/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=crd.alauda.io,resources=alaudaloadbalancer2/finalizers,verbs=update
func (r *ALB2Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	res, err := r.reconcile(ctx, req)
	return res, err
}

func (r *ALB2Reconciler) reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := r.Log
	l.Info("reconcile alb", "alb", req)
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

		if !r.IsV2Alb(alb) {
			l.Info("api version may be not v2beta1,ignore", "object", alb)
			return ctrl.Result{}, nil
		}

		requeue, err := r.HandleBackupAnnotation(ctx, alb)
		if requeue || err != nil {
			return ctrl.Result{Requeue: true}, err
		}

		// set finalizer if not exist
		if !controllerutil.ContainsFinalizer(alb, alb2OperatorFinalizer) {
			l.Info("finalizer not found", "resourceversion", alb.ResourceVersion)
			controllerutil.AddFinalizer(alb, alb2OperatorFinalizer)
			if err := r.Update(ctx, alb); err != nil {
				l.Error(err, "unable to add finalizer on alb2", "alb2", alb)
				return ctrl.Result{}, err
			}
			return ctrl.Result{Requeue: true}, nil
		}

		if !alb.GetDeletionTimestamp().IsZero() {
			l.Info("delete alb", "deletionTimestamp", alb.GetDeletionTimestamp())
			cur, err := depl.LoadAlbDeploy(ctx, r.Client, r.Log, req.NamespacedName)
			if err != nil {
				return ctrl.Result{}, perr.Wrapf(err, "load alb deploy in delete status fail")
			}
			err = depl.Destory(ctx, r.Client, r.Log, cur)
			if err != nil {
				l.Error(err, "clear alb2 subResource", "alb2", alb)
				return ctrl.Result{}, perr.Wrapf(err, "destory alb deploy fail")
			}
			// all clear now.
			controllerutil.RemoveFinalizer(alb, alb2OperatorFinalizer)
			if err := r.Update(ctx, alb); err != nil {
				l.Error(err, "unable to remove finalizer on alb2", "alb2", alb)
				return ctrl.Result{}, perr.Wrapf(err, "remove finalizer,update alb fail")
			}
			// 正常情况下在去掉finalizer之后这个alb就被删除了。重新入队来让他走到not found的状态
			return ctrl.Result{Requeue: true}, nil
		}
	}

	cur, err := depl.LoadAlbDeploy(ctx, r.Client, r.Log, req.NamespacedName)
	if err != nil {
		return ctrl.Result{}, perr.Wrapf(err, "load alb deploy fail")
	}
	conf, err := config.NewALB2Config(cur.Alb, l)
	if err != nil {
		return ctrl.Result{}, perr.Wrapf(err, "load config fail")
	}
	dctl := depl.NewAlbDeployCtl(r.Client, r.Env, r.Log, conf)
	expect, err := dctl.GenExpectAlbDeploy(ctx, cur)
	if err != nil {
		return ctrl.Result{}, perr.Wrapf(err, "gen expect alb deploy fail")
	}
	isReconcile, err := dctl.DoUpdate(ctx, cur, expect)
	if err != nil {
		return ctrl.Result{}, err
	}
	if isReconcile {
		return ctrl.Result{Requeue: true}, nil
	}
	return ctrl.Result{}, nil
}

// TODO FIXME 应该merge到reconcile中
func (r *ALB2Reconciler) OnDeploymentUpdate(ctx context.Context, key types.NamespacedName, curdepl *appsv1.Deployment) error {
	alb := &albv2.ALB2{}
	err := r.Get(ctx, key, alb)
	if err != nil {
		return err
	}
	if alb.Spec.Config == nil {
		r.Log.Info("alb not init yet", "alb", toolkit.ShowMeta(alb))
		return nil
	}
	origin := alb.DeepCopy()
	alb.Status.Detail.Deploy = depl.GenExpectDeployStatus(curdepl)
	depl.MergeAlbStatus(&alb.Status)
	if !reflect.DeepEqual(origin.Status, alb.Status) {
		r.Log.Info("deployment change cause alb status change", "diff", cmp.Diff(origin.Status, alb.Status))
		err := r.Client.Status().Update(ctx, alb)
		return err
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ALB2Reconciler) SetupWithManager(mgr ctrl.Manager) error {
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
					// TODO 应该有个专门的地方来处理status的change
					sameStatus := reflect.DeepEqual(oldObj.Status, newObj.Status)
					if sameSpec && sameLabel && sameAnnotation {
						log.Info("ignore on all same alb.", "status-eq", sameStatus, "diff", cmp.Diff(oldObj, newObj))
						return false
					}
					return true
				},
				GenericFunc: func(genericEvent event.GenericEvent) bool {
					return true
				},
			},
		)).
		Watches(&source.Kind{Type: &appsv1.Deployment{}}, handler.EnqueueRequestsFromMapFunc(r.DeploymentToALbKey),
			builder.WithPredicates(
				predicate.And(predicate.NewPredicateFuncs(func(object client.Object) bool {
					edepl, ok := object.(*appsv1.Deployment)
					if !ok {
						return false
					}
					_, _, version, err := getAlbKeyFromDeployment(edepl)
					if err != nil {
						return false
					}
					if version != r.Env.Version {
						return false
					}
					return true
				}),
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
							edepl, ok := e.ObjectNew.(*appsv1.Deployment)
							if !ok {
								return false
							}
							ns, name, version, err := getAlbKeyFromDeployment(edepl)
							if err != nil {
								return false
							}
							key := types.NamespacedName{Namespace: ns, Name: name}
							r.Log.Info("find deployment update", "key", key, "version", version, "depl", toolkit.ShowMeta(e.ObjectNew))
							err = r.OnDeploymentUpdate(context.Background(), key, edepl)
							if err != nil {
								r.Log.Error(err, "handle deployment change fail")
							}
							return false
						},
						GenericFunc: func(genericEvent event.GenericEvent) bool {
							return false
						},
					}),
			),
		).
		Complete(r)
}

func getAlbKeyFromDeployment(obj client.Object) (ns string, name string, version string, err error) {
	labels := obj.GetLabels()
	key, keyOk := labels[resources.ALB2OperatorResourceLabel]
	version, versionOk := labels[resources.ALB2OperatorVersionLabel]
	if !keyOk || !versionOk {
		return "", "", "", fmt.Errorf("not alb-operator managed deployment")
	}
	ns, name, err = SplitMetaNamespaceKeyWith(key, "_")
	if err != nil {
		return "", "", "", fmt.Errorf("invalid key %s", key)
	}
	return ns, name, version, nil
}

func (r *ALB2Reconciler) DeploymentToALbKey(obj client.Object) []reconcile.Request {
	empty := []reconcile.Request{}
	// not alb-operator managed deployment, ignore.
	ns, name, version, err := getAlbKeyFromDeployment(obj)
	if err != nil {
		// 理论上来讲 这里不应该报错，应该被前面的过滤条件filter掉
		r.Log.Error(err, "deployment to alb fail")
		return []ctrl.Request{}
	}
	if version != r.Env.Version {
		r.Log.Info("version not same,ignore", "depl", toolkit.ShowMeta(obj), "alb2", name, "depl-ver", version, "version", r.Env.Version)
		return empty
	}
	return []reconcile.Request{
		{
			NamespacedName: types.NamespacedName{Namespace: ns, Name: name},
		},
	}
}

func SplitMetaNamespaceKeyWith(key, sep string) (namespace, name string, err error) {
	parts := strings.Split(key, sep)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid key format: %s", key)
	}
	return parts[0], parts[1], nil
}
