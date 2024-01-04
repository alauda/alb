package depl

import (
	"context"

	albv2 "alauda.io/alb2/pkg/apis/alauda/v2beta1"
	. "alauda.io/alb2/pkg/operator/toolkit"
	appsv1 "k8s.io/api/apps/v1"
	coov1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crcli "sigs.k8s.io/controller-runtime/pkg/client"
	gv1b1t "sigs.k8s.io/gateway-api/apis/v1beta1"

	"github.com/go-logr/logr"
	perr "github.com/pkg/errors"

	"alauda.io/alb2/pkg/operator/config"
	"alauda.io/alb2/pkg/operator/controllers/depl/resources/feature"
	"alauda.io/alb2/pkg/operator/controllers/depl/resources/rbac"
	"alauda.io/alb2/pkg/operator/controllers/depl/resources/service"
)

// 从当前集群中获取到正在运行的这个alb的状态
func LoadAlbDeploy(ctx context.Context, cli client.Client, l logr.Logger, req types.NamespacedName, operatorCf config.OperatorCfg) (*AlbDeploy, error) {
	alb := &albv2.ALB2{}
	depl := &appsv1.Deployment{}
	commoncfg := &corev1.ConfigMap{}
	portinfo := &corev1.ConfigMap{}
	ic := &netv1.IngressClass{}
	gc := &gv1b1t.GatewayClass{}
	lease := &coov1.Lease{}
	var err error
	key := crcli.ObjectKey{Namespace: req.Namespace, Name: req.Name}

	// atleast we must have a alb
	err = cli.Get(ctx, key, alb)
	if err != nil {
		return nil, perr.WithMessage(err, "get alb fail when load albdepl")
	}
	l.Info("get current alb deploy", "alb", ShowMeta(alb), "raw", PrettyCr(alb))

	err = cli.Get(ctx, key, depl)
	if errors.IsNotFound(err) {
		depl = nil
	}
	if err != nil && !errors.IsNotFound(err) {
		return nil, perr.WithMessage(err, "get deployment fail when load albdepl")
	}

	// TODO use label deployment的名字可能是不固定的
	err = cli.Get(ctx, key, commoncfg)
	if errors.IsNotFound(err) {
		commoncfg = nil
	}
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	}

	err = cli.Get(ctx, crcli.ObjectKey{Namespace: req.Namespace, Name: req.Name + "-port-info"}, portinfo)
	if errors.IsNotFound(err) {
		portinfo = nil
	}
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	}

	err = cli.Get(ctx, key, ic)
	if errors.IsNotFound(err) {
		ic = nil
	}
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	}

	err = cli.Get(ctx, key, gc)
	if errors.IsNotFound(err) {
		gc = nil
	}
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	}

	fc := feature.NewFeatureCtl(ctx, cli, l)
	fcur, err := fc.Load(key)
	if err != nil {
		return nil, err
	}

	svctl := service.NewSvcCtl(ctx, cli, l, operatorCf)
	svc, err := svctl.Load(key)
	if err != nil {
		return nil, err
	}
	rbac, err := rbac.Load(ctx, cli, l, key)
	if err != nil {
		return nil, err
	}
	err = cli.Get(ctx, key, lease)
	if errors.IsNotFound(err) {
		lease = nil
	}
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	}
	return &AlbDeploy{
		Alb:                alb,
		Deploy:             depl,
		Common:             commoncfg,
		PortInfo:           portinfo,
		IngressClass:       ic,
		SharedGatewayClass: gc,
		Feature:            fcur,
		Svc:                svc,
		Rbac:               rbac,
		Lease:              lease,
	}, nil
}
