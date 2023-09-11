package config

import (
	"bytes"
	"encoding/json"

	. "alauda.io/alb2/pkg/apis/alauda/v2beta1"
	"github.com/xorcare/pointer"
)

func DefaultConfig() ExternalAlbConfig {
	return ExternalAlbConfig{
		LoadbalancerType:     pointer.Of("nginx"),
		NetworkMode:          pointer.Of("host"),
		Replicas:             pointer.Of(3),
		EnableGC:             pointer.Of("false"),
		EnableAlb:            pointer.Of(true),
		EnableGCAppRule:      pointer.Of("false"),
		EnablePrometheus:     pointer.Of("true"),
		EnablePortprobe:      pointer.Of("true"),
		EnablePortProject:    pointer.Of(false),
		NodeSelector:         map[string]string{},
		EnableIPV6:           pointer.Of("true"),
		EnableHTTP2:          pointer.Of("true"),
		EnableIngress:        pointer.Of("true"),
		DefaultIngressClass:  pointer.Of(false),
		EnableCrossClusters:  pointer.Of("false"),
		EnableGzip:           pointer.Of("true"),
		DefaultSSLCert:       pointer.Of(""),
		DefaultSSLStrategy:   pointer.Of("Never"),
		IngressHTTPPort:      pointer.Of(80),
		IngressHTTPSPort:     pointer.Of(443),
		IngressController:    pointer.Of("alb2"),
		MetricsPort:          pointer.Of(1936),
		EnableGoMonitor:      pointer.Of(false),
		EnableProfile:        pointer.Of(false),
		GoMonitorPort:        pointer.Of(1937),
		WorkerLimit:          pointer.Of(8),
		ResyncPeriod:         pointer.Of(300),
		SyncPolicyInterval:   pointer.Of(1),
		CleanMetricsInterval: pointer.Of(2592000),
		Backlog:              pointer.Of(2048),
		MaxTermSeconds:       pointer.Of(30),
		PolicyZip:            pointer.Of(false),
		Gateway:              &ExternalGateway{},
		Resources: &ExternalResources{
			ExternalResource: &ExternalResource{
				Limits: &ContainerResource{
					CPU:    "2",
					Memory: "2Gi",
				},
				Requests: &ContainerResource{
					CPU:    "50m",
					Memory: "128Mi",
				},
			},
			Alb: &ExternalResource{
				Limits: &ContainerResource{
					CPU:    "200m",
					Memory: "2Gi",
				},
				Requests: &ContainerResource{
					CPU:    "50m",
					Memory: "128Mi",
				},
			},
		},
		Projects:        []string{},
		PortProjects:    pointer.Of("[]"),
		AntiAffinityKey: pointer.Of("local"),
		BindNIC:         pointer.Of("{}"),
	}
}

// 在给crd上cfg设置了默认值之后，我们可以直接访问任意的字段而不用判断是否为nil
func NewExternalAlbConfigWithDefault(alb *ALB2) (ExternalAlbConfig, error) {
	var err error
	defaultCfg := DefaultConfig()
	// raw := alb.Spec.Config.Raw
	// https://stackoverflow.com/a/47396406
	// anyway,give two config,we need merge it into one.
	if err != nil {
		return ExternalAlbConfig{}, err
	}
	cfgRaw, err := json.Marshal(alb.Spec.Config)
	if err != nil {
		return ExternalAlbConfig{}, err
	}
	// 因为我们用了omitempty，所以没写的字段会被忽略，所以就可以达到merge的效果
	err = json.NewDecoder(bytes.NewReader(cfgRaw)).Decode(&defaultCfg)
	fixedAlb := defaultCfg
	if err != nil {
		return ExternalAlbConfig{}, err
	}
	// operator 应该保证spec和config没有冲突的地方
	fixedAlb.LoadbalancerName = &alb.Name
	// TODO 对于resource 如果没有设置的话，我们是期望他是unlimited?
	return fixedAlb, nil
}
