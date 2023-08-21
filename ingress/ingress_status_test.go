package ingress

import (
	"reflect"
	"testing"

	"alauda.io/alb2/utils"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	n1 "k8s.io/api/networking/v1"
)

func TestParseAddress(t *testing.T) {
	type testcase struct {
		address  string
		ip       []string
		hostname []string
	}
	cases := []testcase{
		{
			address:  "127.0.0.1,,",
			ip:       []string{"127.0.0.1"},
			hostname: []string{},
		},
		{
			address:  "127.0.0.1,2004::192:168:128:191,alauda.com",
			ip:       []string{"127.0.0.1", "2004::192:168:128:191"},
			hostname: []string{"alauda.com"},
		},
	}
	for _, c := range cases {
		ip, host := utils.ParseAddressStr(c.address)
		assert.Equal(t, c.ip, ip)
		assert.Equal(t, c.hostname, host)
	}
}

func TestFillupIngressStatus(t *testing.T) {
	type testcase struct {
		address      AlbAddress
		port         []int32
		ing          n1.Ingress
		status       n1.IngressStatus
		shouldUpdate bool
	}
	cases := []testcase{
		{
			// 增加127.0.0.1
			address: AlbAddress{ips: []string{"127.0.0.1"}, hosts: []string{"alauda.com"}},
			port:    []int32{80},
			ing: n1.Ingress{
				Status: n1.IngressStatus{
					LoadBalancer: n1.IngressLoadBalancerStatus{
						Ingress: []n1.IngressLoadBalancerIngress{
							{
								Hostname: "alauda.com",
								Ports: []n1.IngressPortStatus{
									{
										Port:     80,
										Protocol: "TCP",
									},
								},
							},
						},
					},
				},
			},
			shouldUpdate: true,
			status: n1.IngressStatus{
				LoadBalancer: n1.IngressLoadBalancerStatus{
					Ingress: []n1.IngressLoadBalancerIngress{
						{
							Hostname: "alauda.com",
							Ports: []n1.IngressPortStatus{
								{
									Port:     80,
									Protocol: "TCP",
								},
							},
						},
						{
							IP: "127.0.0.1",
							Ports: []n1.IngressPortStatus{
								{
									Port:     80,
									Protocol: "TCP",
								},
							},
						},
					},
				},
			},
		},
		{
			// 增加127.0.0.1的443端口
			address: AlbAddress{ips: []string{"127.0.0.1"}},
			port:    []int32{80, 443},
			ing: n1.Ingress{
				Status: n1.IngressStatus{
					LoadBalancer: n1.IngressLoadBalancerStatus{
						Ingress: []n1.IngressLoadBalancerIngress{
							{
								IP: "127.0.0.1",
								Ports: []n1.IngressPortStatus{
									{
										Port:     80,
										Protocol: "TCP",
									},
								},
							},
							{
								IP: "127.0.0.2",
								Ports: []n1.IngressPortStatus{
									{
										Port:     80,
										Protocol: "TCP",
									},
								},
							},
						},
					},
				},
			},
			shouldUpdate: true,
			status: n1.IngressStatus{
				LoadBalancer: n1.IngressLoadBalancerStatus{
					Ingress: []n1.IngressLoadBalancerIngress{
						{
							IP: "127.0.0.1",
							Ports: []n1.IngressPortStatus{
								{
									Port:     80,
									Protocol: "TCP",
								},
								{
									Port:     443,
									Protocol: "TCP",
								},
							},
						},
						{
							IP: "127.0.0.2",
							Ports: []n1.IngressPortStatus{
								{
									Port:     80,
									Protocol: "TCP",
								},
							},
						},
					},
				},
			},
		},
	}
	for _, c := range cases {
		update := FillupIngressStatus(c.address, c.port, &c.ing)
		assert.Equal(t, c.shouldUpdate, update)
		assert.True(t, reflect.DeepEqual(c.status, c.ing.Status), cmp.Diff(c.status, c.ing.Status))
	}
}
