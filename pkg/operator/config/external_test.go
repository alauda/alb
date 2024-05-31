package config

import (
	"fmt"
	"reflect"
	"testing"

	. "alauda.io/alb2/pkg/apis/alauda/v2beta1"
	"github.com/kr/pretty"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/xorcare/pointer"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestParseCPU(t *testing.T) {
	// external_test.go:21: {{2000 -3} {<nil>}  DecimalSI} <nil> 2
	// external_test.go:23: {{2 0} {<nil>} 2 DecimalSI} <nil> 2
	cpu1, err := resource.ParseQuantity("2000m")
	t.Log(cpu1, err, cpu1.String())
	cpu2, err := resource.ParseQuantity("2")
	t.Log(cpu2, err, cpu2.String())
	assert.NotEqual(t, cpu1, cpu2)
	assert.Equal(t, cpu1.String(), cpu2.String())
	t.Log(cpu1.Equal(cpu2), reflect.DeepEqual(cpu1, cpu2))
}

func TestExternalConfigDefaultAndMerge(t *testing.T) {
	{
		rawStr := `
        loadbalancerName: global-alb2
        antiAffinityKey: system
        defaultSSLCert: cpaas-system/dex.tls
        defaultSSLStrategy: Both
        interval: 1
        ingressHTTPPort: 80
        ingressHTTPSPort: 443
        metricsPort: 11782
        nodeSelector:
            cpaas-system-alb: ""
        ingress: "true"
        projects:
            - cpaas-system
        replicas: 3
        resources:
          limits:
            cpu: 210m
            memory: 256Mi
        global:
           external: "ignore and not throw err"
`
		albcfg := &ExternalAlbConfig{}
		_ = yaml.Unmarshal([]byte(rawStr), albcfg)
		alb := &ALB2{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "global-alb2",
				Namespace: "cpaas-system",
			},
			Spec: ALB2Spec{
				Address: "",
				Type:    "",
				Config:  albcfg,
			},
		}
		cfg, err := NewExternalAlbConfigWithDefault(alb)
		assert.NoError(t, err)
		fmt.Printf("actual addr %v \n%# v", alb.Spec.Address, pretty.Formatter(cfg))
		assert.Equal(t, ExternalAlbConfig{
			LoadbalancerName: pointer.Of("global-alb2"), // overwrite
			NodeSelector: map[string]string{
				"cpaas-system-alb": "",
			},
			LoadbalancerType:     pointer.Of("nginx"),
			Replicas:             pointer.Of(3),
			EnablePrometheus:     pointer.Of("true"),
			EnablePortprobe:      pointer.Of("true"),
			EnablePortProject:    pointer.Of(false),
			EnableIPV6:           pointer.Of("true"),
			NetworkMode:          pointer.Of("host"),
			EnableAlb:            pointer.Of(true),
			EnableHTTP2:          pointer.Of("true"),
			EnableIngress:        pointer.Of("true"),
			EnableCrossClusters:  pointer.Of("false"),
			EnableGzip:           pointer.Of("true"),
			DefaultSSLCert:       pointer.Of("cpaas-system/dex.tls"), // overwrite
			DefaultSSLStrategy:   pointer.Of("Both"),
			IngressHTTPPort:      pointer.Of(80),
			IngressHTTPSPort:     pointer.Of(443),
			IngressController:    pointer.Of("alb2"),
			MetricsPort:          pointer.Of(11782),
			EnableGoMonitor:      pointer.Of(false),
			EnableProfile:        pointer.Of(false),
			GoMonitorPort:        pointer.Of(1937),
			WorkerLimit:          pointer.Of(8),
			ResyncPeriod:         pointer.Of(300),
			Interval:             pointer.Of(1),
			ReloadTimeout:        pointer.Of(600),
			SyncPolicyInterval:   pointer.Of(1),
			CleanMetricsInterval: pointer.Of(2592000),
			Backlog:              pointer.Of(2048),
			MaxTermSeconds:       pointer.Of(30),
			PolicyZip:            pointer.Of(false),
			DefaultIngressClass:  pointer.Of(false),
			Gateway:              &ExternalGateway{},
			Resources: &ExternalResources{
				Alb: &ExternalResource{
					Limits:   &ContainerResource{CPU: "200m", Memory: "2Gi"},
					Requests: &ContainerResource{CPU: "50m", Memory: "128Mi"},
				},
				ExternalResource: &ExternalResource{
					Limits:   &ContainerResource{CPU: "210m", Memory: "256Mi"}, // 用户配置的
					Requests: &ContainerResource{CPU: "50m", Memory: "128Mi"},  // 默认的
				},
			},
			Projects:        []string{"cpaas-system"},
			PortProjects:    pointer.Of("[]"),
			AntiAffinityKey: pointer.Of("system"),
			BindNIC:         pointer.Of("{}"),
		}, cfg)
	}
}

func TestDefaultAndMergeResource(t *testing.T) {
	cases := []struct {
		rawStr         string
		expectResource ExternalResources
	}{
		{
			rawStr: `
resources:
    limits:
       cpu: 210m
       memory: 256Mi
`,
			expectResource: ExternalResources{
				Alb: &ExternalResource{
					Limits:   &ContainerResource{CPU: "200m", Memory: "2Gi"},
					Requests: &ContainerResource{CPU: "50m", Memory: "128Mi"},
				},
				ExternalResource: &ExternalResource{
					Limits:   &ContainerResource{CPU: "210m", Memory: "256Mi"},
					Requests: &ContainerResource{CPU: "50m", Memory: "128Mi"},
				},
			},
		},
		{
			rawStr: `
resources:
    limits:
       memory: 257Mi
`,
			expectResource: ExternalResources{
				Alb: &ExternalResource{
					Limits:   &ContainerResource{CPU: "200m", Memory: "2Gi"},
					Requests: &ContainerResource{CPU: "50m", Memory: "128Mi"},
				},
				ExternalResource: &ExternalResource{
					Limits:   &ContainerResource{CPU: "2", Memory: "257Mi"},
					Requests: &ContainerResource{CPU: "50m", Memory: "128Mi"},
				},
			},
		},
		{
			rawStr: `
resources:
    limits:
       memory: ""
`,
			expectResource: ExternalResources{
				Alb: &ExternalResource{
					Limits:   &ContainerResource{CPU: "200m", Memory: "2Gi"},
					Requests: &ContainerResource{CPU: "50m", Memory: "128Mi"},
				},
				ExternalResource: &ExternalResource{
					Limits:   &ContainerResource{CPU: "2", Memory: "2Gi"},
					Requests: &ContainerResource{CPU: "50m", Memory: "128Mi"},
				},
			},
		},
		{
			rawStr: `
resources:
    limits: {}
`,
			expectResource: ExternalResources{
				Alb: &ExternalResource{
					Limits:   &ContainerResource{CPU: "200m", Memory: "2Gi"},
					Requests: &ContainerResource{CPU: "50m", Memory: "128Mi"},
				},
				ExternalResource: &ExternalResource{
					Limits:   &ContainerResource{CPU: "2", Memory: "2Gi"},
					Requests: &ContainerResource{CPU: "50m", Memory: "128Mi"},
				},
			},
		},
	}
	for _, c := range cases {
		albcfg := &ExternalAlbConfig{}
		_ = yaml.Unmarshal([]byte(c.rawStr), albcfg)
		alb := &ALB2{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "global-alb2",
				Namespace: "cpaas-system",
			},
			Spec: ALB2Spec{
				Address: "",
				Type:    "",
				Config:  albcfg,
			},
		}
		cfg, err := NewExternalAlbConfigWithDefault(alb)
		assert.NoError(t, err)
		assert.Equal(t, c.expectResource, *cfg.Resources)
	}
}
