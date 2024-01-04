package standalone_gateway

import (
	"context"
	"fmt"
	_ "log"
	"strings"
	"time"

	albv2 "alauda.io/alb2/pkg/apis/alauda/v2beta1"
	"alauda.io/alb2/pkg/operator/config"
	"alauda.io/alb2/utils"
	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	"github.com/openlyinc/pointy"
	"github.com/samber/lo"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	_ "sigs.k8s.io/controller-runtime/pkg/client"
	ctrcli "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	. "alauda.io/alb2/pkg/operator/controllers/types"
	. "alauda.io/alb2/pkg/operator/toolkit"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrlBuilder "sigs.k8s.io/controller-runtime/pkg/builder"
	gv1b1t "sigs.k8s.io/gateway-api/apis/v1beta1"
)

type GatewayReconciler struct {
	CliExt
	OperatorCf config.OperatorCfg
	Log        logr.Logger
}

func NewGatewayReconciler(cli ctrcli.Client, cfg config.OperatorCfg, log logr.Logger) *GatewayReconciler {
	return &GatewayReconciler{
		CliExt: CliExt{
			Client: cli,
			log:    log,
			cfg:    cfg,
		},
		OperatorCf: cfg,
		Log:        log,
	}
}

func (r *GatewayReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Log.Info("set up gateway reconcile")

	b := ctrl.NewControllerManagedBy(mgr).
		For(&gv1b1t.Gateway{}, builder.WithPredicates(
			predicate.And(
				r.ignoreNotSharedGateway(),
			),
		))
	// 当容器网络模式的alb发生变化时（pending=>running）应该去更新对应的gateway的状态
	r.watchAlb(b)
	return b.Complete(r)
}

func (r *GatewayReconciler) ignoreNotSharedGateway() predicate.Funcs {
	ignore := func(object ctrcli.Object) bool {
		g, ok := object.(*gv1b1t.Gateway)
		if !ok {
			return false
		}
		class := string(g.Spec.GatewayClassName)
		process := class == STAND_ALONE_GATEWAY_CLASS
		return process
	}
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return ignore(e.Object)
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldgw := e.ObjectOld.(*gv1b1t.Gateway)
			newgw := e.ObjectNew.(*gv1b1t.Gateway)
			if oldgw.Spec.GatewayClassName != STAND_ALONE_GATEWAY_CLASS && newgw.Spec.GatewayClassName != STAND_ALONE_GATEWAY_CLASS {
				return false
			}
			return true
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return ignore(e.Object)
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return ignore(e.Object)
		},
	}
}

func (r *GatewayReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	res, err := r.reconcile(ctx, req)
	if err != nil {
		r.Log.Error(err, "reconcile fail")
	}
	return res, err
}

func (r *GatewayReconciler) gatewayclassChange(ctx context.Context, g *gv1b1t.Gateway) (ctrl.Result, error) {
	_, err := r.gatewayDeleted(ctx, ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: g.Namespace, Name: g.Name},
	})
	if err != nil {
		return ctrl.Result{}, err
	}
	delete(g.Labels, fmt.Sprintf(FMT_ALB_REF, r.OperatorCf.BaseDomain))
	err = r.Client.Update(ctx, g)
	if err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *GatewayReconciler) reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := r.Log
	l.Info("reconcile gateway", "gateway", req)
	g, err := r.GetGateway(ctx, req)
	if k8serrors.IsNotFound(err) {
		return r.gatewayDeleted(ctx, req)
	}
	if err != nil {
		return ctrl.Result{}, err
	}
	if g.Spec.GatewayClassName != STAND_ALONE_GATEWAY_CLASS {
		return r.gatewayclassChange(ctx, g)
	}

	l.Info("find gateway", "gateway", PrettyCr(g))
	// 如果gateway上有label了.那么应该就是有对应的alb了的.
	alb, err := r.getAlbFromLabel(ctx, g)
	if err != nil {
		return ctrl.Result{}, err
	}

	if alb != nil {
		err := r.updateGatewayStatus(ctx, g, alb)
		return ctrl.Result{}, err
	}

	// 根据gateway的cr上的配置尝试找到这个alb,如果确实没有的话,就自己创建一个
	alb, exist, err := r.findOrbuildAlb(ctx, g)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !exist {
		err := r.Client.Create(ctx, alb)
		if err != nil {
			return ctrl.Result{}, err
		}
	}
	if g.Labels == nil {
		g.Labels = map[string]string{}
	}
	// 更新gateway上的label
	g.Labels[fmt.Sprintf(FMT_ALB_REF, r.OperatorCf.BaseDomain)] = alb.Name
	err = r.Client.Update(ctx, g)
	if err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *GatewayReconciler) getAlbFromLabel(ctx context.Context, g *gv1b1t.Gateway) (alb *albv2.ALB2, err error) {
	albName := getRefedAlb(g.Labels, r.OperatorCf.BaseDomain)
	r.log.Info("getalbfromlabel", "name", albName, "g", client.ObjectKeyFromObject(g))
	if albName == "" {
		return nil, nil
	}

	alb = &albv2.ALB2{}
	err = r.Get(ctx, ctrcli.ObjectKey{Namespace: g.Namespace, Name: albName}, alb)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			// alb been deleted or cache not syncd?
			return nil, fmt.Errorf("gateway refed alb been deleted? req %v err %v", albName, err)
		}
		return nil, err
	}
	return alb, nil
}

func getRefedAlb(labels map[string]string, domain string) string {
	return labels[fmt.Sprintf(FMT_ALB_REF, domain)]
}

func (r *GatewayReconciler) findOrbuildAlb(ctx context.Context, g *gv1b1t.Gateway) (*albv2.ALB2, bool, error) {
	// list all alb and find the one match the gateway
	albs, err := r.findAlb(ctx, g.Namespace, g.Name)
	if err != nil {
		return nil, false, err
	}
	if len(albs) != 0 {
		return albs[0], true, nil
	}
	return r.buildAlb(g), false, nil
}

func (r *GatewayReconciler) findAlb(ctx context.Context, ns string, name string) ([]*albv2.ALB2, error) {
	albLit := &albv2.ALB2List{}
	albs := []*albv2.ALB2{}
	err := r.List(ctx, albLit, &ctrcli.ListOptions{Namespace: ns})
	if err != nil {
		return nil, err
	}
	for i, alb := range albLit.Items {
		if alb.Spec.Config == nil {
			continue
		}
		if alb.Spec.Config.Gateway == nil {
			continue
		}
		if alb.Spec.Config.Gateway.Mode == nil {
			continue
		}
		if *alb.Spec.Config.Gateway.Mode == albv2.GatewayModeStandAlone {
			if alb.Spec.Config.Gateway.Name == nil {
				continue
			}
			if *alb.Spec.Config.Gateway.Name == name {
				albs = append(albs, &albLit.Items[i])
			}
		}
	}
	return albs, nil
}

func (r *GatewayReconciler) buildAlb(g *gv1b1t.Gateway) *albv2.ALB2 {
	mode := albv2.GatewayModeStandAlone
	network := albv2.CONTAINER_MODE
	return &albv2.ALB2{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-lb-%s", g.Name, strings.ToLower(utils.RandomStr("", 5))),
			Namespace: g.Namespace,
		},
		Spec: albv2.ALB2Spec{
			Type: "nginx",
			Config: &albv2.ExternalAlbConfig{
				NetworkMode: &network,
				Replicas:    pointy.Int(1),
				Gateway: &albv2.ExternalGateway{
					Mode: &mode,
					Name: pointy.String(g.Name),
				},
			},
		},
	}
}

func (r *GatewayReconciler) gatewayDeleted(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.log.Info("a gateway been deleted", "req", req)
	albs, err := r.findAlb(ctx, req.Namespace, req.Name)
	if err != nil {
		return ctrl.Result{}, err
	}
	if len(albs) == 0 {
		r.log.Info("could not find correspond alb, ignore", "req", req)
		return ctrl.Result{}, nil
	}
	for _, alb := range albs {
		r.log.Info("delete gateway cause alb delete", "alb", PrettyCr(alb))
		err := r.Delete(ctx, alb)
		if err != nil {
			r.log.Info("delete alb fail alb, ignore")
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

func (r *GatewayReconciler) watchAlb(b *ctrl.Builder) {
	alb := albv2.ALB2{}
	isAndHasGateway := func(alb *albv2.ALB2) bool {
		albconf, err := config.NewALB2Config(alb, r.OperatorCf, r.log)
		if err != nil {
			r.log.Error(err, "get config fail")
			return false
		}
		gcfg := albconf.Gateway.StandAlone
		if gcfg == nil {
			return false
		}
		// 必须在有gateway的情况下才能去reconcile，否则会被gateway的reconcile认为是一个不存在gateway，会走到删除的逻辑了
		ctx, _ := context.WithTimeout(context.Background(), time.Minute*5)
		key := client.ObjectKey{Namespace: gcfg.GatewayNS, Name: gcfg.GatewayName}
		g := &gv1b1t.Gateway{}
		err = r.Get(ctx, key, g)
		if k8serrors.IsNotFound(err) {
			return false
		}
		if err != nil {
			r.log.Error(err, "get gateway fail")
			return false
		}
		return true
	}

	predicate := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			alb, ok := e.Object.(*albv2.ALB2)
			if !ok {
				return false
			}
			return isAndHasGateway(alb)
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			old, ok := e.ObjectOld.(*albv2.ALB2)
			if !ok {
				return false
			}
			new, ok := e.ObjectNew.(*albv2.ALB2)
			if !ok {
				return false
			}
			if !isAndHasGateway(new) {
				return false
			}
			r.log.Info("gateway watch alb", "alb change", cmp.Diff(old.Status, new.Status))
			statusChange := old.Status.State != new.Status.State
			return statusChange
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return false
		},
	}
	options := ctrlBuilder.WithPredicates(predicate)

	eventhandler := handler.EnqueueRequestsFromMapFunc(func(o client.Object) (ret []reconcile.Request) {
		alb, ok := o.(*albv2.ALB2)
		if !ok {
			return ret
		}
		if !isAndHasGateway(alb) {
			return ret
		}
		albconf, err := config.NewALB2Config(alb, r.OperatorCf, r.log)
		if err != nil {
			r.log.Error(err, "get config fail")
			return ret
		}
		gwcfg := albconf.Gateway.StandAlone
		if gwcfg == nil {
			r.log.Info("could not find gateway cfg?", "cfg", albconf)
			return ret
		}
		key := types.NamespacedName{Namespace: gwcfg.GatewayNS, Name: gwcfg.GatewayName}
		return []reconcile.Request{
			{
				NamespacedName: key,
			},
		}
	})
	b.Watches(&source.Kind{Type: &alb}, eventhandler, options)
}

func (r *GatewayReconciler) updateGatewayStatus(ctx context.Context, g *gv1b1t.Gateway, alb *albv2.ALB2) error {
	albReady := alb.Status.State == albv2.ALB2StateRunning
	albErrMsg := alb.Status.Reason
	gwReady := false
	gwOriginMsg := ""
	cond, index, find := lo.FindIndexOf(g.Status.Conditions, func(c metav1.Condition) bool {
		return c.Type == string(gv1b1t.GatewayConditionAccepted)
	})
	if find {
		gwReady = cond.Status == metav1.ConditionTrue
		gwOriginMsg = cond.Message
	}
	// 只有当alb not ready时，operator才去更新gateway的状态，当alb ready了，alb会自己更新自己的gateway的状态的
	needUpdate := !find || (find && !albReady && !(gwReady == albReady && gwOriginMsg == albErrMsg))

	r.log.Info("check gateway status", "need", needUpdate, "albready", albReady, "orgin", gwOriginMsg, "alb-msg", albErrMsg, "alb", alb.Status)
	if needUpdate {
		// 和前端保持兼容
		g.Status.Conditions[index].LastTransitionTime = metav1.Now()
		g.Status.Conditions[index].Status = metav1.ConditionUnknown
		g.Status.Conditions[index].Reason = string(gv1b1t.GatewayReasonPending)
		g.Status.Conditions[index].Message = albErrMsg
		return r.Status().Update(ctx, g)
	}
	return nil
}
