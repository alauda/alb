package service

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	a2t "alauda.io/alb2/pkg/apis/alauda/v2beta1"
	cfg "alauda.io/alb2/pkg/operator/config"
	. "alauda.io/alb2/pkg/operator/controllers/depl/util"
	. "alauda.io/alb2/pkg/operator/toolkit"
	"alauda.io/alb2/utils"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
	crcli "sigs.k8s.io/controller-runtime/pkg/client"
)

// 约束
// 1. alb的监控信息需要有一个有service_name=alb-$albname的svc
// 2. 容器网络的alb需要有一个lb类型的svc,并且只在vip.enableLbSvc=true的时候才会自己创建
// 3. lb类型的svc必需要根据ft来实时更新这个svc的端口 gateway的容器网络模式下没有ft要根据gatway的listener来更新
// 4. 一个svc必须最少有一个端口
// 5. alb的端口可能是tcp/udp的,在1.23之前，lbsvc只能是相同协议的，1.23上要手动开MixedProtocolLBService，1.24及其之后，是默认开启的

// 所以
// operator创建两个svc,一个和之前的svc保持一致，用来给监控用。一个是lb类型的svc
// operator这里负责lbsvc的创建和销毁，lb的svc的更新由每个alb自己来做
// 使用mixedprotocolsvc的特性，直接在svc上创不同协议的端口
// svc上必须有一个端口，默认的端口是alb的监控的端口

type SvcCtl struct {
	ctx context.Context
	cli crcli.Client
	log logr.Logger
	cfg cfg.OperatorCfg
}

type CurSvc struct {
	MonitorSvc *corev1.Service
	LbSvc      *corev1.Service
}

type SvcUpdate struct {
	svc    *corev1.Service
	action string
}

type SvcUpdates []SvcUpdate

func (s CurSvc) GetObjs() []crcli.Object {
	ret := []crcli.Object{}
	if s.MonitorSvc != nil {
		ret = append(ret, s.MonitorSvc)
	}
	if s.LbSvc != nil {
		ret = append(ret, s.LbSvc)
	}
	return ret
}

func NewSvcCtl(ctx context.Context, cli crcli.Client, log logr.Logger, cfg cfg.OperatorCfg) *SvcCtl {
	return &SvcCtl{
		ctx: ctx,
		cli: cli,
		log: log,
		cfg: cfg,
	}
}

// load current service about this alb
func (s *SvcCtl) Load(albkey crcli.ObjectKey) (CurSvc, error) {
	monitorSvcKey := albkey

	monitorSvc, err := s.getMonitorSvc(monitorSvcKey)
	if err != nil {
		return CurSvc{}, err
	}
	lbSvc, err := s.getLbSvc(albkey)
	if err != nil {
		return CurSvc{}, err
	}
	s.log.Info("load cur svc", "monitorSvc", PrettyCr(monitorSvc))
	s.log.Info("load cur svc", "lbSvc", PrettyCr(lbSvc))
	return CurSvc{
		MonitorSvc: monitorSvc,
		LbSvc:      lbSvc,
	}, nil
}

func (s *SvcCtl) genMonitorSvcUpdate(cur CurSvc, cf cfg.Config, alb *a2t.ALB2) SvcUpdate {
	name := cf.ALB.Name
	ns := cf.ALB.Ns
	metrics := int32(cf.ALB.Controller.MetricsPort)
	svcType := "ClusterIP"
	if IsNil(cur.MonitorSvc) {
		svc := &corev1.Service{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Service",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: ns,
			},
			// service必须有一个port
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{
						Name:     "metrics",
						Protocol: "TCP",
						Port:     metrics,
						TargetPort: intstr.IntOrString{
							Type:   intstr.Int,
							IntVal: metrics,
						},
					},
				},
				Type:            corev1.ServiceType(svcType),
				SessionAffinity: "None",
			},
		}
		s.patchMonitorSvcDefaultConfig(svc, alb, cf)
		return SvcUpdate{
			svc:    svc,
			action: "create",
		}
	}
	// 已有一个svc检查是否要更新
	svc := cur.MonitorSvc.DeepCopy()
	if s.patchMonitorSvcDefaultConfig(svc, alb, cf) {
		return SvcUpdate{
			action: "update",
			svc:    svc,
		}
	}
	return SvcUpdate{action: "none"}
}

func (s *SvcCtl) genLbSvcUpdate(cur CurSvc, cf cfg.Config, alb *a2t.ALB2) SvcUpdate {
	// 关闭开关的情况
	if !cf.ALB.Vip.EnableLbSvc && !IsNil(cur.LbSvc) {
		return SvcUpdate{
			action: "delete",
			svc:    cur.LbSvc,
		}
	}

	if !cf.ALB.Vip.EnableLbSvc && IsNil(cur.LbSvc) {
		return SvcUpdate{
			action: "none",
			svc:    cur.LbSvc,
		}
	}

	// 创建
	if cf.ALB.Vip.EnableLbSvc && IsNil(cur.LbSvc) {
		metrics := int32(cf.ALB.Controller.MetricsPort)
		svc := &corev1.Service{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Service",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-lb-%s", cf.ALB.Name, strings.ToLower(utils.RandomStr("", 5))),
				Namespace: cf.ALB.Ns,
			},
			// service必须有一个port
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{
						Name:     "metrics",
						Protocol: "TCP",
						Port:     metrics,
						TargetPort: intstr.IntOrString{
							Type:   intstr.Int,
							IntVal: metrics,
						},
					},
				},
				Type: corev1.ServiceType("LoadBalancer"),
			},
		}
		s.patchLbSvcDefaultConfig(svc, alb, cf.ALB)
		return SvcUpdate{
			svc:    svc,
			action: "create",
		}
	}
	// 检查更新
	svc := cur.LbSvc.DeepCopy()
	if s.patchLbSvcDefaultConfig(svc, alb, cf.ALB) {
		return SvcUpdate{
			action: "update",
			svc:    svc,
		}
	}
	return SvcUpdate{action: "none"}
}

func (s *SvcCtl) GenUpdate(cur CurSvc, cf cfg.Config, alb *a2t.ALB2) SvcUpdates {
	return SvcUpdates{
		s.genMonitorSvcUpdate(cur, cf, alb),
		s.genLbSvcUpdate(cur, cf, alb),
	}
}

func (s *SvcCtl) patchMonitorSvcDefaultConfig(svc *corev1.Service, alb *a2t.ALB2, cfg cfg.Config) (dirty bool) {
	origin := svc.DeepCopy()
	owner := MakeOwnerRefs(alb)
	svc.OwnerReferences = owner
	ns := alb.Namespace
	name := alb.Name
	refLabel := ALB2ResourceLabel(ns, name, cfg.Operator.Version)
	// 监控的svc依赖这个label
	label := map[string]string{
		"service_name": "alb2-" + name,
	}
	svc.Labels = MergeMap(refLabel, label)
	svc.Spec.Selector = label
	return !reflect.DeepEqual(origin, svc)
}

func (s *SvcCtl) patchLbSvcDefaultConfig(svc *corev1.Service, alb *a2t.ALB2, albcfg cfg.ALB2Config) (dirty bool) {
	origin := svc.DeepCopy()
	owner := MakeOwnerRefs(alb)
	svc.OwnerReferences = owner
	name := alb.Name
	cfg := s.cfg
	refLabel := ALB2ResourceLabel(alb.Namespace, name, cfg.Version)
	if svc.Annotations == nil {
		svc.Annotations = map[string]string{}
	}
	// merge annotation
	// 当lbsvcannotation 从 a:1,b:1变成 a:1时，需要把b:1删掉,这里在svc上记一个origin-annotation来维护这个状态
	for _, k := range s.findNeedDeleteAnnotation(svc.Annotations, albcfg.Vip.LbSvcAnnotations) {
		delete(svc.Annotations, k)
	}

	jsonstr, _ := json.Marshal(albcfg.Vip.LbSvcAnnotations)
	svc.Annotations[s.originAnnotationKey()] = string(jsonstr)
	for k, v := range albcfg.Vip.LbSvcAnnotations {
		svc.Annotations[k] = v
	}
	// add label which will be used when load
	svc.Labels = MergeMap(refLabel, LbSvcLabel(crcli.ObjectKeyFromObject(alb), s.cfg.BaseDomain))
	svc.Spec.Type = corev1.ServiceTypeLoadBalancer
	policy := corev1.IPFamilyPolicyPreferDualStack
	svc.Spec.IPFamilyPolicy = &policy
	// 这个svc也是指向alb的
	svc.Spec.Selector = map[string]string{"service_name": "alb2-" + name}
	enable := true
	if albcfg.Vip.AllocateLoadBalancerNodePorts != nil {
		enable = *albcfg.Vip.AllocateLoadBalancerNodePorts
	}
	svc.Spec.AllocateLoadBalancerNodePorts = &enable
	return !reflect.DeepEqual(origin, svc)
}

func (s *SvcCtl) findNeedDeleteAnnotation(origin map[string]string, new map[string]string) []string {
	val, ok := origin[s.originAnnotationKey()]
	if !ok {
		return []string{}
	}
	originCfg := map[string]string{}
	json.Unmarshal([]byte(val), &originCfg)
	return mapset.NewSetFromMapKeys(originCfg).Difference(mapset.NewSetFromMapKeys(new)).ToSlice()
}

func (s *SvcCtl) getLbSvc(key crcli.ObjectKey) (*corev1.Service, error) {
	// 在升级时,会更新svc的label,所以在获取时要先判断下,拿到旧的label,让他去更新
	{
		legacy, err := s.getLbSvcLegacy(key)
		if err != nil {
			return nil, err
		}
		if legacy != nil {
			return legacy, err
		}
	}

	svcs := corev1.ServiceList{}
	err := s.cli.List(s.ctx, &svcs, &crcli.ListOptions{LabelSelector: labels.SelectorFromSet(LbSvcLabel(key, s.cfg.BaseDomain))})
	if err != nil {
		return nil, err
	}
	if len(svcs.Items) == 0 {
		return nil, nil
	}
	return &svcs.Items[0], nil
}

func (s *SvcCtl) getLbSvcLegacy(key crcli.ObjectKey) (*corev1.Service, error) {
	svcs := corev1.ServiceList{}
	err := s.cli.List(s.ctx, &svcs, &crcli.ListOptions{LabelSelector: labels.SelectorFromSet(LegacyLbSvcLabel(key, s.cfg.BaseDomain))})
	if err != nil {
		return nil, err
	}
	if len(svcs.Items) == 0 {
		return nil, nil
	}
	return &svcs.Items[0], nil
}

func LbSvcLabel(key crcli.ObjectKey, basedomain string) map[string]string {
	return MergeMap(
		ALBLabel(key.Namespace, key.Name),
		map[string]string{
			serviceTypeKey(basedomain): "lb",
		},
	)
}

func LegacyLbSvcLabel(key crcli.ObjectKey, basedomain string) map[string]string {
	return MergeMap(
		LegacyALBLabel(key.Namespace, key.Name),
		map[string]string{
			serviceTypeKey(basedomain): "lb",
		},
	)
}

func serviceTypeKey(domain string) string {
	return fmt.Sprintf("alb.%s/service_type", domain)
}

func (s *SvcCtl) originAnnotationKey() string {
	return fmt.Sprintf("alb.%s/origin_annotation", s.cfg.BaseDomain)
}

func (s *SvcCtl) DoUpdate(ac SvcUpdates) error {
	for _, svc := range ac {
		origin := svc.svc.DeepCopy()
		var err error
		if svc.action == "create" {
			err = s.cli.Create(s.ctx, svc.svc, &crcli.CreateOptions{})
		}
		if svc.action == "update" {
			err = s.cli.Update(s.ctx, svc.svc, &crcli.UpdateOptions{})
		}
		if svc.action == "delete" {
			err = s.cli.Delete(s.ctx, svc.svc, &crcli.DeleteOptions{})
		}
		if err != nil {
			s.log.Info("update svc fail", "action", svc.action, "svc", PrettyCr(svc.svc))
			return err
		}
		s.log.Info("update svc success", "action", svc.action, "diff", cmp.Diff(origin, svc.svc))
	}
	return nil
}

func (s *SvcCtl) getMonitorSvc(key crcli.ObjectKey) (*corev1.Service, error) {
	svc := &corev1.Service{}
	err := s.cli.Get(s.ctx, key, svc)
	if errors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return svc, nil
}

func (s *CurSvc) Show() string {
	return PrettyCr(s.MonitorSvc) + "\n" + PrettyCr(s.LbSvc)
}
