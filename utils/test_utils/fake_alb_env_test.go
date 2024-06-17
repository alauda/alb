package test_utils

import (
	"testing"

	albv2 "alauda.io/alb2/pkg/apis/alauda/v2beta1"
	"github.com/stretchr/testify/assert"
	k8sv1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestFakeEnv(t *testing.T) {
	albName := "alb-1"
	defaultAlbs := []albv2.ALB2{
		{
			ObjectMeta: k8smetav1.ObjectMeta{
				Namespace: DEFAULT_NS,
				Name:      albName,
			},
			Spec: albv2.ALB2Spec{
				Type: "nginx",
				Config: &albv2.ExternalAlbConfig{
					Projects: []string{"ALL_ALL", "project-1"},
				},
			},
		},
	}

	defaultNamespaces := []k8sv1.Namespace{
		{
			ObjectMeta: k8smetav1.ObjectMeta{
				Name:   DEFAULT_NS,
				Labels: map[string]string{"alauda.io/project": "random-project"},
			},
		},
	}
	res := FakeResource{
		Alb: FakeALBResource{
			Albs: defaultAlbs,
		},
		K8s: FakeK8sResource{
			IngressesClass: []networkingv1.IngressClass{
				{
					ObjectMeta: k8smetav1.ObjectMeta{
						Name: "alb2",
					},
					Spec: networkingv1.IngressClassSpec{
						Controller: "alauda.io/alb2",
					},
				},
			},
			Namespaces: defaultNamespaces,
		},
	}
	env := NewFakeEnv()
	env.AssertStart()
	err := env.ApplyFakes(res)
	assert.NoError(t, err)
	err = env.ClearFakes(res)
	assert.NoError(t, err)
	env.Stop()
}
