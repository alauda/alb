package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"runtime"
	"testing"
	"text/template"

	albv1 "alauda.io/alb2/pkg/apis/alauda/v1"
	"alauda.io/alb2/utils"
	"alauda.io/alb2/utils/test_utils"
	"github.com/stretchr/testify/assert"
	k8sv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestPolicies_Less(t *testing.T) {
	type args struct {
		i int
		j int
	}
	tests := []struct {
		name string
		p    Policies
		args args
		want bool
	}{
		{
			"1",
			[]*Policy{
				{Priority: 100, RawPriority: 5},
				{Priority: 100, RawPriority: 5},
			},
			args{0, 1},
			false,
		},
		{
			"2",
			[]*Policy{
				{Priority: 100, RawPriority: 4},
				{Priority: 100, RawPriority: 5},
			},
			args{0, 1},
			true,
		},
		{
			"3",
			[]*Policy{
				{Priority: 99, RawPriority: 5},
				{Priority: 100, RawPriority: 5},
			},
			args{0, 1},
			false,
		},
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.p.Less(tt.args.i, tt.args.j); got != tt.want {
				t.Errorf("Less() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGenerateNginxConfig(t *testing.T) {

	fakeResource := test_utils.FakeResource{
		Alb: test_utils.FakeALBResource{
			Albs: []albv1.ALB2{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "alb-1",
						Namespace: "ns-1",
					},
					Spec: albv1.ALB2Spec{
						Address: "1.2.3.4",
						Type:    "nginx",
					},
				},
			},
			Frontends: []albv1.Frontend{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "ft-1",
						Namespace: "ns-1",
						Labels: map[string]string{
							"alb2.alauda.io/name": "alb-1",
						},
					},
					Spec: albv1.FrontendSpec{
						Port:     8000,
						Protocol: "http",
						Source: &albv1.Source{
							Name:      "ft-source-1",
							Namespace: "ns-1",
							Type:      "ingress",
						},
						ServiceGroup: &albv1.ServiceGroup{
							Services: []albv1.Service{
								{
									Name:      "ft-service-1",
									Namespace: "ns-1",
								},
							},
						},
					},
				},
			},
			Rules: []albv1.Rule{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "rule-1",
						Namespace: "ns-1",
						Labels: map[string]string{
							"alb2.alauda.io/name":     "alb-1",
							"alb2.alauda.io/frontend": "ft-1",
						},
					},
					Spec: albv1.RuleSpec{
						DSLX: albv1.DSLX{
							{
								Values: [][]string{{utils.OP_REGEX, "^/v1/*"}},
								Type:   utils.KEY_URL,
							},
							{
								Values: [][]string{{utils.OP_EQ, "alauda.io"}},
								Type:   utils.KEY_HOST,
							},
						},
						ServiceGroup: &albv1.ServiceGroup{
							Services: []albv1.Service{
								{
									Name:      "s-1",
									Namespace: "ns-1",
									Port:      8080,
								},
							},
						},
						Source: &albv1.Source{
							Name:      "rule-source-1",
							Namespace: "ns-1",
							Type:      "ingress",
						},
					},
				},
			},
		},
		K8s: test_utils.FakeK8sResource{
			Services: []k8sv1.Service{
				{

					ObjectMeta: metav1.ObjectMeta{
						Name:      "ft-service-1",
						Namespace: "ns-1",
					},
					Spec: k8sv1.ServiceSpec{
						Type: k8sv1.ServiceTypeClusterIP,
						Ports: []k8sv1.ServicePort{
							{Port: 8080, TargetPort: intstr.FromInt(8080)},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "s-1",
						Namespace: "ns-1",
					},
					Spec: k8sv1.ServiceSpec{
						Type: k8sv1.ServiceTypeClusterIP,
						Ports: []k8sv1.ServicePort{
							{Port: 8080, TargetPort: intstr.FromInt(8080)},
						},
					},
				},
			},
			EndPoints: []k8sv1.Endpoints{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "s-1",
						Namespace: "ns-1",
					},
					Subsets: []k8sv1.EndpointSubset{
						{
							Addresses: []k8sv1.EndpointAddress{
								{
									IP:       "192.168.10.3",
									Hostname: "s-1-ep-1",
								},
							},
						},
					},
				},
			},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	a := assert.New(t)
	defer cancel()
	drv, _ := test_utils.InitFakeAlb(t, ctx, fakeResource, test_utils.DEFAULT_CONFIG_FOR_TEST)

	ctl := NewNginxController(drv)
	nginxConfig, nginxPolicy, err := ctl.GenerateNginxConfigAndPolicy()
	a.NoError(err)
	nginxConfigStr, err := renderNginxConfig(nginxConfig)
	a.NoError(err)
	nginxPolicyJson, err := json.MarshalIndent(nginxPolicy, " ", " ")
	nginxPolicyJsonStr := string(nginxPolicyJson)
	a.NoError(err)
	a.Contains(nginxConfigStr, "8000")
	a.Contains(nginxPolicyJsonStr, "192.168.10.3")
	a.NoError(err)
}

func TestRenderNginxConfig(t *testing.T) {
	config := NginxTemplateConfig{
		Frontends: map[int]FtConfig{
			8081: {
				Port:            8081,
				Protocol:        "http",
				IpV4BindAddress: []string{"192.168.0.1", "192.168.0.3"},
				IpV6BindAddress: []string{"[::1]", "[::2]"},
			},
		},
		NginxParam: NginxParam{EnableIPV6: true},
	}
	configStr, err := renderNginxConfig(config)
	assert.Nil(t, err)
	assert.Contains(t, configStr, "listen     192.168.0.1:8081")
	assert.Contains(t, configStr, "listen     192.168.0.3:8081")
	assert.Contains(t, configStr, "listen     [::1]:8081")
	assert.Contains(t, configStr, "listen     [::2]:8081")
}

func renderNginxConfig(config NginxTemplateConfig) (string, error) {
	// get the current test file abs path
	_, filename, _, _ := runtime.Caller(0)
	t, err := template.New("nginx.tmpl").ParseFiles(fmt.Sprintf("%s/../template/nginx/nginx.tmpl", filepath.Dir(filename)))
	var tpl bytes.Buffer
	if err != nil {
		return "", err
	}

	if err := t.Execute(&tpl, config); err != nil {
		return "", err
	}

	return tpl.String(), nil
}
