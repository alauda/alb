package service

import (
	"testing"

	a2t "alauda.io/alb2/pkg/apis/alauda/v2beta1"
	cfg "alauda.io/alb2/pkg/operator/config"

	. "alauda.io/alb2/utils/test_utils"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestUpdateLbSvcAnnotation(t *testing.T) {
	tests := []struct {
		name              string
		svc               *corev1.Service
		alb               *a2t.ALB2
		cf                cfg.Config
		expectAnnotations map[string]string
	}{
		{
			svc: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test",
					Annotations: map[string]string{},
				},
			},
			alb: &a2t.ALB2{},
			cf: cfg.Config{
				ALB: cfg.ALB2Config{
					Vip: a2t.VipConfig{
						EnableLbSvc: true,
						LbSvcAnnotations: map[string]string{
							"a": "b",
						},
					},
				},
				Operator: cfg.OperatorCfg{
					BaseDomain: "cpaas.io",
				},
			},
			expectAnnotations: map[string]string{
				"alb.cpaas.io/origin_annotation": "{\"a\":\"b\"}",
				"a":                              "b",
			},
		},
		{
			svc: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					Annotations: map[string]string{
						"alb.cpaas.io/origin_annotation": "{\"b\":\"1\"}",
						"b":                              "1",
					},
				},
			},
			alb: &a2t.ALB2{},
			cf: cfg.Config{
				ALB: cfg.ALB2Config{
					Vip: a2t.VipConfig{
						EnableLbSvc: true,
						LbSvcAnnotations: map[string]string{
							"a": "b",
						},
					},
				},
				Operator: cfg.OperatorCfg{
					BaseDomain: "cpaas.io",
				},
			},
			expectAnnotations: map[string]string{
				"alb.cpaas.io/origin_annotation": "{\"a\":\"b\"}",
				"a":                              "b",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewSvcCtl(nil, nil, ConsoleLog(), tt.cf.Operator)
			svc.patchLbSvcDefaultConfig(tt.svc, tt.alb, tt.cf.ALB)
			assert.Equal(t, tt.expectAnnotations, tt.svc.Annotations)
		})
	}
}
