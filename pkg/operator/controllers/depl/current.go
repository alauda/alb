package depl

import (
	"context"
	"fmt"

	albv2 "alauda.io/alb2/pkg/apis/alauda/v2beta1"
	. "alauda.io/alb2/pkg/operator/toolkit"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctlClient "sigs.k8s.io/controller-runtime/pkg/client"
	gv1b1t "sigs.k8s.io/gateway-api/apis/v1beta1"

	"github.com/go-logr/logr"
	perr "github.com/pkg/errors"
)

//从当前集群中获取到正在运行的这个alb的状态

func LoadAlbDeploy(ctx context.Context, cli client.Client, l logr.Logger, req types.NamespacedName) (*AlbDeploy, error) {
	alb := &albv2.ALB2{}
	depl := &appsv1.Deployment{}
	commoncfg := &corev1.ConfigMap{}
	portinfo := &corev1.ConfigMap{}
	svc := &corev1.Service{}
	tcpsvc := &corev1.Service{}
	udpsvc := &corev1.Service{}
	ic := &netv1.IngressClass{}
	gc := &gv1b1t.GatewayClass{}
	var err error

	// atleast we must have a alb
	err = cli.Get(ctx, ctlClient.ObjectKey{Namespace: req.Namespace, Name: req.Name}, alb)
	if err != nil {
		return nil, perr.WithMessage(err, "get alb fail when load albdepl")
	}
	l.Info("get current alb deploy", "alb", ShowMeta(alb), "raw", PrettyCr(alb))

	err = cli.Get(ctx, ctlClient.ObjectKey{Namespace: req.Namespace, Name: req.Name}, depl)
	if errors.IsNotFound(err) {
		depl = nil
	}
	if err != nil && !errors.IsNotFound(err) {
		return nil, perr.WithMessage(err, "get deployment fail when load albdepl")
	}

	// TODO use label deployment的名字可能是不固定的
	err = cli.Get(ctx, ctlClient.ObjectKey{Namespace: req.Namespace, Name: req.Name}, commoncfg)
	if errors.IsNotFound(err) {
		commoncfg = nil
	}
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	}

	err = cli.Get(ctx, ctlClient.ObjectKey{Namespace: req.Namespace, Name: req.Name + "-port-info"}, portinfo)
	if errors.IsNotFound(err) {
		portinfo = nil
	}
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	}

	err = cli.Get(ctx, ctlClient.ObjectKey{Namespace: req.Namespace, Name: req.Name}, svc)
	if errors.IsNotFound(err) {
		svc = nil
	}
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	}

	err = cli.Get(ctx, ctlClient.ObjectKey{Namespace: req.Namespace, Name: req.Name + "-tcp"}, tcpsvc)
	if errors.IsNotFound(err) {
		tcpsvc = nil
	}
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	}

	err = cli.Get(ctx, ctlClient.ObjectKey{Namespace: req.Namespace, Name: req.Name + "-udp"}, udpsvc)
	if errors.IsNotFound(err) {
		udpsvc = nil
	}
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	}

	err = cli.Get(ctx, ctlClient.ObjectKey{Namespace: req.Namespace, Name: req.Name}, ic)
	if errors.IsNotFound(err) {
		ic = nil
	}
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	}

	err = cli.Get(ctx, ctlClient.ObjectKey{Namespace: req.Namespace, Name: req.Name}, gc)
	if errors.IsNotFound(err) {
		gc = nil
	}
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	}
	feature := EmptyFeatureCr()
	featureKey := ctlClient.ObjectKey{Namespace: "", Name: fmt.Sprintf("%s-%s", req.Name, req.Namespace)}
	err = cli.Get(ctx, featureKey, feature)
	if errors.IsNotFound(err) {
		feature = nil
	}
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	}

	return &AlbDeploy{
		Alb:      alb,
		Deploy:   depl,
		Common:   commoncfg,
		PortInfo: portinfo,
		Ingress:  ic,
		Gateway:  gc,
		Feature:  feature,
		Svc: &AlbDeploySvc{
			Svc:    svc,
			TcpSvc: tcpsvc,
			UdpSvc: udpsvc,
		},
	}, nil
}
