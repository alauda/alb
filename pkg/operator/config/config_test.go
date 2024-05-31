package config

import (
	"sort"
	"testing"

	"alauda.io/alb2/utils/test_utils"

	albv2 "alauda.io/alb2/pkg/apis/alauda/v2beta1"
	. "alauda.io/alb2/pkg/config"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
)

func yamltoAlbCr(rawyaml, ns, name string) *albv2.ALB2 {
	alb := &albv2.ALB2{}
	err := yaml.Unmarshal([]byte(rawyaml), alb)
	if err != nil {
		panic(err)
	}
	return alb
}

func initALB2Instance() *albv2.ALB2 {
	data := `
apiVersion: crd.alauda.io/v2beta1
kind: ALB2
metadata:
    name: test
    namespace: n1
spec:
  address: "127.0.0.1"
  type: "nginx" 
  config:
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
        mode: "standalone"
        name: "name"
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
	return yamltoAlbCr(data, "n1", "test")
}

func TestGetALBContainerEnvs(t *testing.T) {
	alb := initALB2Instance()
	cfg, err := NewALB2Config(alb, DEFAULT_OPERATOR_CFG, test_utils.ConsoleLog())
	assert.NoError(t, err)
	expect := []corev1.EnvVar{
		{
			Name:  "MAX_TERM_SECONDS",
			Value: "30",
		},
		{
			Name:  "ENABLE_PROMETHEUS",
			Value: "true",
		},
		{
			Name:  "ENABLE_PORTPROBE",
			Value: "false",
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
			Value: "false",
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
			Name:  "DEFAULT_SSL_STRATEGY",
			Value: "Never",
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
					APIVersion: "v1",
					FieldPath:  "metadata.name",
				},
			},
		},
		{
			Name:  "CPU_PRESET",
			Value: "2",
		},
		{
			Name:  "NETWORK_MODE",
			Value: "container",
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
			Value: "false",
		},
		{
			Name:  "GATEWAY_ENABLE",
			Value: "true",
		},
		{
			Name:  "GATEWAY_MODE",
			Value: string(albv2.GatewayModeStandAlone),
		},
		{
			Name:  "GATEWAY_NAME",
			Value: "name",
		},
		{
			Name:  "GATEWAY_NS",
			Value: "n1",
		},
		{
			Name:  "RESYNC_PERIOD",
			Value: "300",
		},
		{
			Name:  "METRICS_PORT",
			Value: "1936",
		},
		{
			Name:  "ENABLE_VIP",
			Value: "true",
		},
		{
			Name:  "INTERVAL",
			Value: "5",
		},
		{
			Name:  "RELOAD_TIMEOUT",
			Value: "600",
		},
	}
	t.Log(test_utils.PrettyJson(cfg))
	actual := cfg.GetALBContainerEnvs()
	sort.Slice(actual, func(i, j int) bool {
		return actual[i].Name < actual[j].Name
	})
	sort.Slice(expect, func(i, j int) bool {
		return expect[i].Name < expect[j].Name
	})
	assert.ElementsMatch(t, expect, actual)
	t.Logf("diff %v", cmp.Diff(expect, actual))
}

func TestGetNginxContainerEnvs(t *testing.T) {
	alb := initALB2Instance()
	cfg, err := NewALB2Config(alb, DEFAULT_OPERATOR_CFG, test_utils.ConsoleLog())
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
			Name:  "DEFAULT_SSL_STRATEGY",
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
		{
			Name:  "OLD_CONFIG_PATH",
			Value: "/etc/alb2/nginx/nginx.conf",
		},
		{
			Name: "MY_POD_NAME",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					APIVersion: "v1",
					FieldPath:  "metadata.name",
				},
			},
		},
	}
	actual := cfg.GetNginxContainerEnvs()
	sort.Slice(expect, func(i, j int) bool {
		return expect[i].Name < expect[j].Name
	})
	sort.Slice(actual, func(i, j int) bool {
		return actual[i].Name < actual[j].Name
	})
	t.Logf(cmp.Diff(expect, actual))
	assert.ElementsMatch(t, expect, actual)
}

func TestConfigFromEnv(t *testing.T) {
	t.Logf("test config from env")
}

func TestConfigViaAlbCr(t *testing.T) {
	type TestCase struct {
		albCr  string
		ns     string
		name   string
		assert func(cfg ALB2Config)
	}
	cases := []TestCase{
		{
			albCr: `
apiVersion: crd.alauda.io/v2beta1
kind: ALB2
metadata:
    name: test 
    namespace: n1 
spec:
    address: "127.0.0.1"
    type: "nginx" 
    config:
        gateway:
            enable: true
`,
			ns:   "n1",
			name: "test",
			assert: func(cfg ALB2Config) {
				t.Logf(test_utils.PrettyJson(cfg))
				assert.Equal(t, cfg.Gateway.Enable, true)
				assert.Equal(t, cfg.Gateway.Mode, albv2.GatewayModeShared)
				assert.Equal(t, cfg.Gateway.Shared.GatewayClassName, "test")
				assert.Equal(t, cfg.Gateway.StandAlone == nil, true)
			},
		},
		{
			albCr: `
apiVersion: crd.alauda.io/v2beta1
kind: ALB2
metadata:
    name: test-xxxx
    namespace: n1 
spec:
    address: "127.0.0.1"
    type: "nginx" 
    config:
        gateway:
            mode: "standalone"
            name: "test/n2"
`,
			ns:   "n1",
			name: "test-xxxx",
			assert: func(cfg ALB2Config) {
				t.Logf(test_utils.PrettyJson(cfg))
				assert.Equal(t, cfg.Gateway.Enable, true)
				assert.Equal(t, cfg.Gateway.Mode, albv2.GatewayModeStandAlone)
				assert.Equal(t, cfg.Gateway.StandAlone.GatewayName, "test")
				assert.Equal(t, cfg.Gateway.StandAlone.GatewayNS, "n2")
				assert.Equal(t, cfg.Vip.EnableLbSvc, true)
				assert.Equal(t, cfg.Name, "test-xxxx")
				assert.Equal(t, cfg.Gateway.Shared == nil, true)
			},
		},
		{
			albCr: `
apiVersion: crd.alauda.io/v2beta1
kind: ALB2
metadata:
    name: test-xxxx
    namespace: n1 
spec:
    address: "127.0.0.1"
    type: "nginx" 
    config:
        gateway:
            mode: "standalone"
            name: "test"
`,
			ns:   "n1",
			name: "test-xxxx",
			assert: func(cfg ALB2Config) {
				t.Logf(test_utils.PrettyJson(cfg))
				assert.Equal(t, cfg.Gateway.Enable, true)
				assert.Equal(t, cfg.Gateway.Mode, albv2.GatewayModeStandAlone)
				assert.Equal(t, cfg.Gateway.StandAlone.GatewayName, "test")
				assert.Equal(t, cfg.Gateway.StandAlone.GatewayNS, "n1")
				assert.Equal(t, cfg.Vip.EnableLbSvc, true)
				assert.Equal(t, cfg.Name, "test-xxxx")
				assert.Equal(t, cfg.Gateway.Shared == nil, true)
			},
		},
	}
	for _, c := range cases {
		alb := yamltoAlbCr(c.albCr, c.ns, c.name)
		t.Logf(test_utils.PrettyCr(alb))
		cfg, err := NewALB2Config(alb, DEFAULT_OPERATOR_CFG, test_utils.ConsoleLog())
		assert.NoError(t, err)
		c.assert(*cfg)
		env := map[string]string{}
		for _, e := range cfg.GetALBContainerEnvs() {
			env[e.Name] = e.Value
		}
		ncfg, err := AlbRunCfgFromEnv(env)
		assert.NoError(t, err)
		t.Logf(cmp.Diff(ncfg, cfg.ALBRunConfig))
		assert.Equal(t, cfg.ALBRunConfig, ncfg)
	}
}

func TestToCore(t *testing.T) {
	assert.Equal(t, CpuPresetToCore("100m"), 1)
	assert.Equal(t, CpuPresetToCore("1000m"), 1)
	assert.Equal(t, CpuPresetToCore("1001m"), 2)
	assert.Equal(t, CpuPresetToCore("3000m"), 3)
	assert.Equal(t, CpuPresetToCore("3001m"), 4)
}
