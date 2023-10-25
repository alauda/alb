package controller

import (
	"context"
	"fmt"

	"alauda.io/alb2/config"
	. "alauda.io/alb2/controller/types"
	albv1 "alauda.io/alb2/pkg/apis/alauda/v1"
	opsvc "alauda.io/alb2/pkg/operator/controllers/depl/resources/service"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/google/go-cmp/cmp"
	"github.com/samber/lo"
	apiv1 "k8s.io/api/core/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	crcli "sigs.k8s.io/controller-runtime/pkg/client"
)

// in container mode, we want to create/update loadbalancer tcp/udp service,use it as high avaiable solution.
func (nc *NginxController) SyncLbSvcPort(frontends []*Frontend) error {
	return MixProtocolLbSvc{nc: nc}.sync(nc.Ctx, frontends)
}

func (nc *NginxController) getMetricsPort(serviceProtocol corev1.Protocol) ([]corev1.ServicePort, error) {
	var metricsPorts []corev1.ServicePort
	metricsPort := config.GetConfig().GetMetricsPort()
	if metricsPort > 0 && metricsPort <= 65535 {
		metricsPorts = []corev1.ServicePort{{Name: "metrics", Port: int32(metricsPort), Protocol: serviceProtocol}}
	} else {
		return nil, fmt.Errorf("ENV parameter METRICS_PORT is wrong value as(should be in 1-65535): %v", metricsPort)
	}
	return metricsPorts, nil
}

// LoadBalancer Service could only have one protocol.
var Ft2SvcProtocolMap = map[albv1.FtProtocol]apiv1.Protocol{
	albv1.FtProtocolHTTP:  apiv1.ProtocolTCP,
	albv1.FtProtocolHTTPS: apiv1.ProtocolTCP,
	albv1.FtProtocolgRPC:  apiv1.ProtocolTCP,
	albv1.FtProtocolTCP:   apiv1.ProtocolTCP,
	albv1.FtProtocolUDP:   apiv1.ProtocolUDP,
}

type MixProtocolLbSvc struct {
	nc *NginxController
}

// TODO 现在 operator和alb内使用的client还没有统一
func GetLbSvc(ctx context.Context, cli kubernetes.Interface, key crcli.ObjectKey, domain string) (*corev1.Service, error) {
	sel := labels.SelectorFromSet(opsvc.LbSvcLabel(key, domain)).String()
	ls, err := cli.CoreV1().Services(key.Namespace).List(ctx, metav1.ListOptions{LabelSelector: sel})
	if err != nil {
		return nil, err
	}

	if len(ls.Items) == 0 {
		return nil, k8serrors.NewNotFound(schema.GroupResource{Resource: "service"}, key.Name)
	}
	return &ls.Items[0], nil
}

func (s MixProtocolLbSvc) sync(ctx context.Context, frontends []*Frontend) error {
	nc := s.nc
	cli := nc.Driver
	log := s.nc.log
	log.Info("sync lb svc ports")
	cfg := config.GetConfig()
	ns := cfg.GetNs()
	name := cfg.GetAlbName()
	svc, err := GetLbSvc(ctx, cli.Client, crcli.ObjectKey{Namespace: ns, Name: name}, cfg.GetDomain())
	// 当lb svc不存在时，不做任何事
	if svc == nil || k8serrors.IsNotFound(err) {
		log.Info("svc not find. ignore")
		return nil
	}
	if err != nil {
		return err
	}

	metricsPort := int32(cfg.GetMetricsPort())
	origin := svc.DeepCopy()
	svc.Spec.Ports = []corev1.ServicePort{
		{
			Name:     "metrics",
			Protocol: "TCP",
			Port:     metricsPort,
			TargetPort: intstr.IntOrString{
				Type:   intstr.Int,
				IntVal: metricsPort,
			},
		},
	}
	for _, f := range frontends {
		p, ok := Ft2SvcProtocolMap[f.Protocol]
		if !ok {
			nc.log.Info("frontend port %v, spec.protocol is invalid as value %v", f.Port, f.Protocol)
			continue
		}
		svc.Spec.Ports = append(svc.Spec.Ports, corev1.ServicePort{
			Name:     fmt.Sprintf("%s-%d", f.Protocol, f.Port),
			Protocol: p,
			Port:     int32(f.Port),
			TargetPort: intstr.IntOrString{
				Type:   intstr.Int,
				IntVal: int32(f.Port),
			},
		})
	}

	eq, reason := arrayEq(svc.Spec.Ports, origin.Spec.Ports, func(p corev1.ServicePort) string {
		return fmt.Sprintf("%v-%v-%v-%v", p.Name, p.Protocol, p.Port, p.TargetPort.String())
	})
	if eq {
		return nil
	}
	nsvc, err := cli.Client.CoreV1().Services(ns).Update(ctx, svc, metav1.UpdateOptions{})
	log.Info("update lb svc", "diff", cmp.Diff(svc, nsvc), "reason", reason)
	return err
}

func arrayEq[T any](left []T, right []T, id func(T) string) (bool, string) {
	lm := mapset.NewSet(lo.Map(left, func(x T, _ int) string { return id(x) })...)
	rm := mapset.NewSet(lo.Map(right, func(x T, _ int) string { return id(x) })...)
	if len(left) != len(right) {
		return false, fmt.Sprintf("left len %v right len %v", len(left), len(right))
	}
	return lm.Equal(rm), lm.Difference(rm).String()
}
