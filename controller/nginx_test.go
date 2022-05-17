package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"path/filepath"
	"runtime"
	"testing"
	"text/template"

	"alauda.io/alb2/config"
	. "alauda.io/alb2/controller/types"
	albv1 "alauda.io/alb2/pkg/apis/alauda/v1"
	"alauda.io/alb2/utils"
	"alauda.io/alb2/utils/test_utils"
	"github.com/stretchr/testify/assert"
	k8sv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"k8s.io/klog/v2"
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
		{
			"4",
			[]*Policy{
				{Priority: 100, RawPriority: 100, Rule: "a"},
				{Priority: 100, RawPriority: 100, Rule: "b"},
			},
			args{0, 1},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.p.Less(tt.args.i, tt.args.j); got != tt.want {
				t.Errorf("Less() = %v, want %v", got, tt.want)
			}
		})
	}
}

func GenPolicyAndConfig(t *testing.T, res test_utils.FakeResource) (*NgxPolicy, string, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	drv := test_utils.InitFakeAlb(t, ctx, res, test_utils.DEFAULT_CONFIG_FOR_TEST)
	ctl := NewNginxController(drv)
	nginxConfig, nginxPolicy, err := ctl.GenerateNginxConfigAndPolicy()
	assert.NoError(t, err)
	// marshal and unmarshal to make sure we generate a valid policy json file
	policy := NgxPolicy{}
	nginxPolicyJson, err := json.MarshalIndent(nginxPolicy, " ", " ")
	assert.NoError(t, err)
	err = json.Unmarshal(nginxPolicyJson, &policy)
	assert.NoError(t, err)

	nginxConfigStr, err := renderNginxConfig(nginxConfig)
	assert.NoError(t, err)
	return &policy, nginxConfigStr, nil
}

func (p *NgxPolicy) GetBackendGroup(name string) *BackendGroup {
	for _, be := range p.BackendGroup {
		if be.Name == name {
			return be
		}
	}
	return nil
}

func AssertBackendsEq(t *testing.T, left, right []*Backend) {
	assert.Equal(t, len(left), len(right), "len not eq")
	leftMap := map[int]*Backend{}

	for i, be := range left {
		leftMap[i] = be
	}

	for _, be := range right {
		find := false
		for i, leftBe := range leftMap {
			if leftBe != nil && *be == *leftBe {
				find = true
				leftMap[i] = nil
				break
			}
		}
		assert.True(t, find, "could not find be %v", be)
	}
}

func TestGenerateAlbPolicyAndConfig(t *testing.T) {
	config.Set("METRICS_PORT", "1936")
	config.Set("BACKLOG", "100")
	klog.InitFlags(nil)
	flag.Set("logtostderr", "true")
	defer klog.Flush()
	certAKey, certACert, err := test_utils.GenCert("a.b.c")
	assert.NoError(t, err)
	certBKey, certBCert, err := test_utils.GenCert("c.b.a")
	assert.NoError(t, err)

	type TestCase struct {
		Name   string
		Only   bool
		Skip   bool
		Res    func() test_utils.FakeResource
		Assert func(p NgxPolicy, cfg string)
	}
	run_test := func(cases []TestCase) {
		casesRun := []TestCase{}
		{
			hasOnly := false
			for _, c := range cases {
				if c.Only {
					hasOnly = true
				}
			}
			if hasOnly {
				for _, c := range cases {
					if c.Only && !c.Skip {
						casesRun = append(casesRun, c)
					}
				}
			} else {
				for _, c := range cases {
					if !c.Skip {
						casesRun = append(casesRun, c)
					}
				}
			}
		}

		for _, c := range casesRun {
			t.Logf("run test %s", c.Name)
			albPolicy, ngxCfg, err := GenPolicyAndConfig(t, c.Res())
			assert.NoError(t, err, c.Name)
			c.Assert(*albPolicy, ngxCfg)
		}
	}
	defaultAlb := []albv1.ALB2{
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
	}
	cases := []TestCase{
		{
			// ft port 8000 have 3 rule
			// rule-1 rule-2 have same priority 4, but rule-1 is more complex that rule-2. rule-3 priority is 3, the order should be 3 1 2.
			// rule-1 use svc1Port1 rule-2 use svc1Port2
			Name: "http with different rule and different weight",
			Res: func() test_utils.FakeResource {
				ftPort := 8000
				ftSg1Port := 8001
				svc1Port1 := 8002
				svc1Port2 := 8003
				svc1Port2ContainerPort := 8004
				return test_utils.FakeResource{
					Alb: test_utils.FakeALBResource{
						Albs: defaultAlb,
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
									Port:     albv1.PortNumber(ftPort),
									Protocol: "http",
									ServiceGroup: &albv1.ServiceGroup{
										Services: []albv1.Service{
											{
												Name:      "ft-service-1",
												Namespace: "ns-1",
												Port:      ftSg1Port,
												Weight:    100,
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
									Priority: 4,
									DSLX: albv1.DSLX{
										{
											Values: [][]string{{utils.OP_EQ, "alauda.io"}},
											Type:   utils.KEY_HOST,
										},
										{
											Values: [][]string{{utils.OP_EQ, "/a"}},
											Type:   utils.KEY_URL,
										},
									},
									ServiceGroup: &albv1.ServiceGroup{
										Services: []albv1.Service{
											{
												Name:      "s-1",
												Namespace: "ns-1",
												Port:      svc1Port1,
												Weight:    100,
											},
										},
									},
								},
							},
							{
								ObjectMeta: metav1.ObjectMeta{
									Name:      "rule-2",
									Namespace: "ns-1",
									Labels: map[string]string{
										"alb2.alauda.io/name":     "alb-1",
										"alb2.alauda.io/frontend": "ft-1",
									},
								},
								Spec: albv1.RuleSpec{
									Priority: 4,
									DSLX: albv1.DSLX{
										{
											Values: [][]string{{utils.OP_EQ, "alauda.io.2"}},
											Type:   utils.KEY_HOST,
										},
									},
									ServiceGroup: &albv1.ServiceGroup{
										Services: []albv1.Service{
											{
												Name:      "s-1",
												Namespace: "ns-1",
												Port:      svc1Port1,
												Weight:    50,
											},
											{
												Name:      "s-1",
												Namespace: "ns-1",
												Port:      svc1Port2,
												Weight:    50,
											},
										},
									},
								},
							},
							{
								ObjectMeta: metav1.ObjectMeta{
									Name:      "rule-3",
									Namespace: "ns-1",
									Labels: map[string]string{
										"alb2.alauda.io/name":     "alb-1",
										"alb2.alauda.io/frontend": "ft-1",
									},
								},
								Spec: albv1.RuleSpec{
									Priority: 3,
									DSLX: albv1.DSLX{
										{
											Values: [][]string{{utils.OP_EQ, "alauda.io.3"}},
											Type:   utils.KEY_HOST,
										},
									},
									ServiceGroup: &albv1.ServiceGroup{
										Services: []albv1.Service{
											{
												Name:      "s-1",
												Namespace: "ns-1",
												Port:      svc1Port1,
												Weight:    100,
											},
										},
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
										{Port: int32(ftSg1Port), TargetPort: intstr.FromInt(ftSg1Port), Protocol: k8sv1.ProtocolTCP},
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
										{Name: "p1", Port: int32(svc1Port1), TargetPort: intstr.FromInt(svc1Port1), Protocol: k8sv1.ProtocolTCP},
										{Name: "p2", Port: int32(svc1Port2), TargetPort: intstr.FromString("pp2"), Protocol: k8sv1.ProtocolTCP},
									},
								},
							},
						},
						EndPoints: []k8sv1.Endpoints{
							{
								ObjectMeta: metav1.ObjectMeta{
									Name:      "ft-service-1",
									Namespace: "ns-1",
								},
								Subsets: []k8sv1.EndpointSubset{
									{
										Ports: []k8sv1.EndpointPort{
											{
												Port:     int32(ftSg1Port),
												Protocol: k8sv1.ProtocolTCP,
											},
										},
										Addresses: []k8sv1.EndpointAddress{
											{
												IP:       "192.168.10.4",
												Hostname: "s-1-ep-2",
											},
											{
												IP:       "192.168.10.5",
												Hostname: "s-1-ep-2",
											},
										},
									},
								},
							},
							{
								ObjectMeta: metav1.ObjectMeta{
									Name:      "s-1",
									Namespace: "ns-1",
								},
								Subsets: []k8sv1.EndpointSubset{
									{
										Ports: []k8sv1.EndpointPort{
											{
												Name:     "p1",
												Port:     int32(svc1Port1),
												Protocol: k8sv1.ProtocolTCP,
											},
											{
												Name:     "p2",
												Port:     int32(svc1Port2ContainerPort),
												Protocol: k8sv1.ProtocolTCP,
											},
										},
										Addresses: []k8sv1.EndpointAddress{
											{
												IP:       "192.168.20.3",
												Hostname: "s-1-ep-1",
											},
											{
												IP:       "192.168.20.4",
												Hostname: "s-1-ep-1",
											},
										},
									},
								},
							},
						},
					},
				}
			},
			Assert: func(albPolicy NgxPolicy, ngxCfg string) {
				assert.Equal(t, len(albPolicy.Http.Tcp[8000]), 4)
				assert.Equal(t, len(albPolicy.BackendGroup), 4)
				assert.Contains(t, ngxCfg, "8000")
				assert.Equal(t, albPolicy.Http.Tcp[8000][0].Upstream, "rule-3")
				assert.Equal(t, albPolicy.Http.Tcp[8000][1].Upstream, "rule-1")
				assert.Equal(t, albPolicy.Http.Tcp[8000][2].Upstream, "rule-2")
				assert.Equal(t, albPolicy.Http.Tcp[8000][3].Upstream, "alb-1-8000-http")
				AssertBackendsEq(t, albPolicy.GetBackendGroup("rule-1").Backends, []*Backend{{"192.168.20.3", 8002, 50, "", nil}, {"192.168.20.4", 8002, 50, "", nil}})
				AssertBackendsEq(t, albPolicy.GetBackendGroup("rule-2").Backends, []*Backend{{"192.168.20.3", 8004, 25, "", nil}, {"192.168.20.4", 8004, 25, "", nil}, {"192.168.20.3", 8002, 25, "", nil}, {"192.168.20.4", 8002, 25, "", nil}})
				AssertBackendsEq(t, albPolicy.GetBackendGroup("alb-1-8000-http").Backends, []*Backend{{"192.168.10.4", 8001, 50, "", nil}, {"192.168.10.5", 8001, 50, "", nil}})
			},
		},
		{
			Name: "tcp",
			Res: func() test_utils.FakeResource {
				ftPort := 8000
				ftSg1Port := 8001
				return test_utils.FakeResource{
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
									Port:     albv1.PortNumber(ftPort),
									Protocol: "tcp",
									ServiceGroup: &albv1.ServiceGroup{
										Services: []albv1.Service{
											{
												Name:      "ft-service-1",
												Namespace: "ns-1",
												Port:      ftSg1Port,
												Weight:    100,
											},
										},
									},
								},
							},
						},
						Rules: []albv1.Rule{},
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
										{Port: int32(ftSg1Port), TargetPort: intstr.FromInt(ftSg1Port), Protocol: k8sv1.ProtocolTCP},
									},
								},
							},
						},
						EndPoints: []k8sv1.Endpoints{
							{
								ObjectMeta: metav1.ObjectMeta{
									Name:      "ft-service-1",
									Namespace: "ns-1",
								},
								Subsets: []k8sv1.EndpointSubset{
									{
										Ports: []k8sv1.EndpointPort{
											{
												Port:     int32(ftSg1Port),
												Protocol: k8sv1.ProtocolTCP,
											},
										},
										Addresses: []k8sv1.EndpointAddress{
											{
												IP:       "192.168.10.4",
												Hostname: "s-1-ep-2",
											},
											{
												IP:       "192.168.10.5",
												Hostname: "s-1-ep-2",
											},
										},
									},
								},
							},
						},
					},
				}
			},
			Assert: func(albPolicy NgxPolicy, ngxCfg string) {
				listen, err := test_utils.PickStreamServerListen(ngxCfg)
				assert.NoError(t, err)
				assert.Equal(t, listen, []string{"0.0.0.0:8000", "[::]:8000"})
				policies := albPolicy.Stream.Tcp[8000]
				assert.Equal(t, len(policies), 1)
				assert.Equal(t, policies[0].Upstream, "alb-1-8000-tcp")
				AssertBackendsEq(t, albPolicy.GetBackendGroup("alb-1-8000-tcp").Backends, []*Backend{{"192.168.10.4", 8001, 50, "", nil}, {"192.168.10.5", 8001, 50, "", nil}})
			},
		},
		{
			Name: "udp",
			Res: func() test_utils.FakeResource {
				ftPort := 8000
				ftSg1Port := 8001
				return test_utils.FakeResource{
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
									Port:     albv1.PortNumber(ftPort),
									Protocol: "udp",
									ServiceGroup: &albv1.ServiceGroup{
										Services: []albv1.Service{
											{
												Name:      "ft-service-1",
												Namespace: "ns-1",
												Port:      ftSg1Port,
												Weight:    100,
											},
										},
									},
								},
							},
						},
						Rules: []albv1.Rule{},
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
										{Port: int32(ftSg1Port), TargetPort: intstr.FromInt(ftSg1Port), Protocol: k8sv1.ProtocolTCP},
										{Port: int32(ftSg1Port), TargetPort: intstr.FromInt(ftSg1Port), Protocol: k8sv1.ProtocolUDP},
									},
								},
							},
						},
						EndPoints: []k8sv1.Endpoints{
							{
								ObjectMeta: metav1.ObjectMeta{
									Name:      "ft-service-1",
									Namespace: "ns-1",
								},
								Subsets: []k8sv1.EndpointSubset{
									{
										Ports: []k8sv1.EndpointPort{
											{
												Port:     int32(ftSg1Port),
												Protocol: k8sv1.ProtocolTCP,
											},
											{
												Port:     int32(ftSg1Port),
												Protocol: k8sv1.ProtocolUDP,
											},
										},
										Addresses: []k8sv1.EndpointAddress{
											{
												IP:       "192.168.10.4",
												Hostname: "s-1-ep-2",
											},
											{
												IP:       "192.168.10.5",
												Hostname: "s-1-ep-2",
											},
										},
									},
								},
							},
						},
					},
				}
			},
			Assert: func(albPolicy NgxPolicy, ngxCfg string) {
				listen, err := test_utils.PickStreamServerListen(ngxCfg)
				assert.NoError(t, err)
				assert.Equal(t, listen, []string{"0.0.0.0:8000 udp", "[::]:8000 udp"})

				policies := albPolicy.Stream.Udp[8000]
				assert.Equal(t, len(policies), 1)
				assert.Equal(t, policies[0].Upstream, "alb-1-8000-udp")
				AssertBackendsEq(t, albPolicy.GetBackendGroup("alb-1-8000-udp").Backends, []*Backend{{"192.168.10.4", 8001, 50, "", nil}, {"192.168.10.5", 8001, 50, "", nil}})
			},
		},
		{
			Name: "alb have both 80 http,443 https,53 udp and 53 tcp",
			Res: func() test_utils.FakeResource {
				return test_utils.FakeResource{
					Alb: test_utils.FakeALBResource{
						Albs: defaultAlb,
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
									Port:     53,
									Protocol: "tcp",
									ServiceGroup: &albv1.ServiceGroup{
										Services: []albv1.Service{
											{
												Name:      "ft-service-1",
												Namespace: "ns-1",
												Port:      53,
												Weight:    100,
											},
										},
									},
								},
							},
							{
								ObjectMeta: metav1.ObjectMeta{
									Name:      "ft-1-udp",
									Namespace: "ns-1",
									Labels: map[string]string{
										"alb2.alauda.io/name": "alb-1",
									},
								},
								Spec: albv1.FrontendSpec{
									Port:     53,
									Protocol: "udp",
									ServiceGroup: &albv1.ServiceGroup{
										Services: []albv1.Service{
											{
												Name:      "ft-service-1",
												Namespace: "ns-1",
												Port:      53,
												Weight:    100,
											},
										},
									},
								},
							},
							{
								ObjectMeta: metav1.ObjectMeta{
									Name:      "ft-2",
									Namespace: "ns-1",
									Labels: map[string]string{
										"alb2.alauda.io/name": "alb-1",
									},
								},
								Spec: albv1.FrontendSpec{
									Port:     80,
									Protocol: "http",
									ServiceGroup: &albv1.ServiceGroup{
										Services: []albv1.Service{
											{
												Name:      "ft-service-1",
												Namespace: "ns-1",
												Port:      53,
												Weight:    100,
											},
										},
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
										{Port: 53, TargetPort: intstr.FromInt(53), Protocol: k8sv1.ProtocolTCP},
										{Port: 53, TargetPort: intstr.FromInt(53), Protocol: k8sv1.ProtocolUDP},
									},
								},
							},
						},
						EndPoints: []k8sv1.Endpoints{
							{
								ObjectMeta: metav1.ObjectMeta{
									Name:      "ft-service-1",
									Namespace: "ns-1",
								},
								Subsets: []k8sv1.EndpointSubset{
									{
										Ports: []k8sv1.EndpointPort{
											{
												Port:     53,
												Protocol: k8sv1.ProtocolTCP,
											},
											{
												Port:     53,
												Protocol: k8sv1.ProtocolUDP,
											},
										},
										Addresses: []k8sv1.EndpointAddress{
											{
												IP:       "192.168.10.4",
												Hostname: "s-1-ep-2",
											},
											{
												IP:       "192.168.10.5",
												Hostname: "s-1-ep-2",
											},
										},
									},
								},
							},
						},
					},
				}
			},
			Assert: func(p NgxPolicy, cfg string) {
				listen, err := test_utils.PickHttpServerListen(cfg)
				assert.NoError(t, err)
				assert.Equal(t, listen, []string{"0.0.0.0:1936", "[::]:1936", "0.0.0.0:80 backlog=100 default_server", "[::]:80 backlog=100 default_server"})
				listen, err = test_utils.PickStreamServerListen(cfg)
				assert.NoError(t, err)
				assert.Equal(t, listen, []string{"0.0.0.0:53", "[::]:53", "0.0.0.0:53 udp", "[::]:53 udp"})
			},
		},
		{
			Name: "https cert",
			Res: func() test_utils.FakeResource {
				return test_utils.FakeResource{
					Alb: test_utils.FakeALBResource{
						Albs: defaultAlb,
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
									Port:            443,
									Protocol:        "https",
									CertificateName: "ns-1/cert-a",
									ServiceGroup: &albv1.ServiceGroup{
										Services: []albv1.Service{
											{
												Name:      "ft-service-1",
												Namespace: "ns-1",
												Port:      80,
												Weight:    100,
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
									Priority:        4,
									Domain:          "a.b.c",
									CertificateName: "ns-1/cert-b",
									DSLX: albv1.DSLX{
										{
											Values: [][]string{{utils.OP_EQ, "alauda.io"}},
											Type:   utils.KEY_HOST,
										},
										{
											Values: [][]string{{utils.OP_EQ, "/a"}},
											Type:   utils.KEY_URL,
										},
									},
									ServiceGroup: &albv1.ServiceGroup{
										Services: []albv1.Service{
											{
												Name:      "ft-service-1",
												Namespace: "ns-1",
												Port:      80,
												Weight:    100,
											},
										},
									},
								},
							},
						},
					},
					K8s: test_utils.FakeK8sResource{
						Secrets: []k8sv1.Secret{
							{
								ObjectMeta: metav1.ObjectMeta{
									Name:      "cert-a",
									Namespace: "ns-1",
								},
								Data: map[string][]byte{
									k8sv1.TLSPrivateKeyKey: []byte(certAKey),
									k8sv1.TLSCertKey:       []byte(certACert),
								},
							},
							{
								ObjectMeta: metav1.ObjectMeta{
									Name:      "cert-b",
									Namespace: "ns-1",
								},
								Data: map[string][]byte{
									k8sv1.TLSPrivateKeyKey: []byte(certBKey),
									k8sv1.TLSCertKey:       []byte(certBCert),
								},
							},
						},
						Services: []k8sv1.Service{
							{
								ObjectMeta: metav1.ObjectMeta{
									Name:      "ft-service-1",
									Namespace: "ns-1",
								},
								Spec: k8sv1.ServiceSpec{
									Type: k8sv1.ServiceTypeClusterIP,
									Ports: []k8sv1.ServicePort{
										{Port: int32(80), TargetPort: intstr.FromInt(80), Protocol: k8sv1.ProtocolTCP},
									},
								},
							},
						},
						EndPoints: []k8sv1.Endpoints{
							{
								ObjectMeta: metav1.ObjectMeta{
									Name:      "ft-service-1",
									Namespace: "ns-1",
								},
								Subsets: []k8sv1.EndpointSubset{
									{
										Ports: []k8sv1.EndpointPort{
											{
												Port:     int32(80),
												Protocol: k8sv1.ProtocolTCP,
											},
										},
										Addresses: []k8sv1.EndpointAddress{
											{
												IP:       "192.168.10.4",
												Hostname: "s-1-ep-2",
											},
											{
												IP:       "192.168.10.5",
												Hostname: "s-1-ep-2",
											},
										},
									},
								},
							},
						},
					},
				}
			},
			Assert: func(p NgxPolicy, cfg string) {
				assert.Equal(t, len(p.CertificateMap), 2)
				assert.Equal(t, p.CertificateMap["443"].Key, certAKey)
				assert.Equal(t, p.CertificateMap["443"].Cert, certACert)
				assert.Equal(t, p.CertificateMap["a.b.c"].Key, certBKey)
				assert.Equal(t, p.CertificateMap["a.b.c"].Cert, certBCert)
			},
		},
	}

	run_test(cases)
}

func TestRenderNginxConfig(t *testing.T) {
	config := NginxTemplateConfig{
		Frontends: map[string]FtConfig{
			"8081-http": {
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
