package config

import (
	"fmt"
	"testing"

	"alauda.io/alb2/utils/test_utils"

	albv2 "alauda.io/alb2/pkg/apis/alauda/v2beta1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
)

func initALB2Instance() *albv2.ALB2 {

	data := `
    networkMode: host
    loadbalancerName: test 
    address: 127.0.0.1
    nodeSelector:
        a: b
    replicas: 3
    enableGC: "false"
    enableGCAppRule: "true"
    enablePrometheus: "true"
    enableGoMonitor: true
    enableProfile: false
    resyncPeriod: 300
    displayName: ""
    antiAffinityKey: "local"
    enablePortprobe: "true"
    enablePortProject: false
    projects: ["ALL_ALL"]
    enableCrossClusters: "false"
    maxTermSeconds: 30
    portProjects: '[{"port":"113-333","projects":["p1"]}]'
    bindNIC: "{}"
    policyZip: false # both
    enableIngress: "true"
    defaultSSLCert: ""
    defaultSSLStrategy: Never
    ingressHTTPPort: 80
    ingressHTTPSPort: 443
    ingressController: "alb2"
    gateway:
        enable: true
        mode: "gateway"
        gatewayModeCfg:
            name: "ns/name"
    enableIPV6: "true"
    enableHTTP2: "true"
    enableGzip: "true"
    metricsPort: 1936
    goMonitorPort: 1937
    workerLimit: 8
    syncPolicyInterval: 1
    cleanMetricsInterval: 2592001
    backlog: 2048
    cpuLimit: 2
    resources:
      limits:
        cpu: "2"
        memory: 2Gi
      requests:
        cpu: 50m
        memory: 128Mi
    loadbalancerType: nginx
`
	object := &albv2.ExternalAlbConfig{}
	err := yaml.Unmarshal([]byte(data), &object)
	if err != nil {
		panic(err)
	}
	return &albv2.ALB2{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "n1",
		},
		Spec: albv2.ALB2Spec{
			Config: object,
		},
	}
}

func TestGetALBContainerEnvs(t *testing.T) {
	alb := initALB2Instance()
	cfg, err := NewALB2Config(alb, test_utils.ConsoleLog())
	assert.NoError(t, err)
	expect := []corev1.EnvVar{
		{
			Name:  "MAX_TERM_SECONDS",
			Value: "30",
		},
		{
			Name:  "ENABLE_GC",
			Value: "false",
		},
		{
			Name:  "ENABLE_GC_APP_RULE",
			Value: "true",
		},
		{
			Name:  "ENABLE_PROMETHEUS",
			Value: "true",
		},
		{
			Name:  "ENABLE_PORTPROBE",
			Value: "true",
		},
		{
			Name:  "ENABLE_IPV6",
			Value: "true",
		},
		{
			Name:  "ENABLE_HTTP2",
			Value: "true",
		},
		{
			Name:  "ENABLE_GZIP",
			Value: "true",
		},
		{
			Name:  "ENABLE_GO_MONITOR",
			Value: "true",
		},
		{
			Name:  "ENABLE_PROFILE",
			Value: "false",
		},
		{
			Name:  "GO_MONITOR_PORT",
			Value: "1937",
		},
		{
			Name:  "BACKLOG",
			Value: "2048",
		},
		{
			Name:  "POLICY_ZIP",
			Value: "false",
		},
		{
			Name:  "SERVE_CROSSCLUSTERS",
			Value: "false",
		},
		{
			Name:  "SERVE_INGRESS",
			Value: "true",
		},
		{
			Name:  "WORKER_LIMIT",
			Value: "8",
		},
		{
			Name:  "DEFAULT-SSL-CERTIFICATE",
			Value: "",
		},
		{
			Name:  "DEFAULT-SSL-STRATEGY",
			Value: "Never",
		},
		{
			Name:  "MODE",
			Value: "controller",
		},
		{
			Name:  "NAMESPACE",
			Value: "n1",
		},
		{
			Name:  "NAME",
			Value: "test", // 用cr的name
		},
		{
			Name: "MY_POD_NAME",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.name",
				},
			},
		},
		{
			Name:  "CPU_PRESET",
			Value: "2",
		},
		{
			Name:  "NETWORK_MODE",
			Value: "host",
		},
		{
			Name:  "DOMAIN",
			Value: "cpaas.io",
		},
		{
			Name:  "INGRESS_HTTP_PORT",
			Value: "80",
		},
		{
			Name:  "INGRESS_HTTPS_PORT",
			Value: "443",
		},
		{
			Name:  "ALB_ENABLE",
			Value: "true",
		},
		{
			Name:  "GATEWAY_ENABLE",
			Value: "true",
		},
		{
			Name:  "GATEWAY_MODE",
			Value: "gateway",
		},
		{
			Name:  "GATEWAY_NAME",
			Value: "ns/name",
		},
		{
			Name:  "RESYNC_PERIOD",
			Value: "300",
		},
		{
			Name:  "METRICS_PORT",
			Value: "1936",
		},
	}
	actual := cfg.GetALBContainerEnvs(DEFAULT_OPERATOR_CFG)
	for _, e := range actual {
		fmt.Printf("env %v\n", e)
	}
	assert.ElementsMatch(t, expect, actual)
}

func TestGetNginxContainerEnvs(t *testing.T) {
	alb := initALB2Instance()
	cfg, err := NewALB2Config(alb, test_utils.ConsoleLog())
	if err != nil {
		t.Error(err)
	}
	expect := []corev1.EnvVar{
		{
			Name:  "MAX_TERM_SECONDS",
			Value: "30",
		},
		{
			Name:  "SYNC_POLICY_INTERVAL",
			Value: "1",
		},
		{
			Name:  "CLEAN_METRICS_INTERVAL",
			Value: "2592000",
		},
		{
			Name:  "DEFAULT-SSL-STRATEGY",
			Value: "Never",
		},
		{
			Name:  "INGRESS_HTTPS_PORT",
			Value: "443",
		},
		{
			Name:  "POLICY_ZIP",
			Value: "false",
		},
		{
			Name:  "NEW_POLICY_PATH",
			Value: "/etc/alb2/nginx/policy.new",
		},
	}
	actual := cfg.GetNginxContainerEnvs()
	assert.ElementsMatch(t, expect, actual)
}
