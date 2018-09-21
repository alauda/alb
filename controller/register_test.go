package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNeedUpdate(t *testing.T) {
	a := assert.New(t)
	lb := &LoadBalancer{
		Name: "test-lb",
		Frontends: []*Frontend{
			&Frontend{
				Port:     80,
				Protocol: "http",
				Rules: RuleList{
					&Rule{
						Domain: "svc1.ns1.test-lb-alauda.myalauda.cn",
						Type:   "system",
					},
					&Rule{
						Domain: "svc2.ns2.test-lb-alauda.myalauda.cn",
						Type:   "system",
					},
				},
			},
		},
	}
	testcase := []struct {
		listeners []*Listener
		expection bool
	}{
		{
			listeners: []*Listener{
				&Listener{
					ContainerPort: 80,
					ListenerPort:  80,
					Protocol:      "http",
					Domains:       []string{"svc.ns.{LB_DOMAINS}"},
				},
			},
			expection: true,
		},
		{
			listeners: []*Listener{
				&Listener{
					ContainerPort: 80,
					ListenerPort:  80,
					Protocol:      "http",
					Domains: []string{
						"svc1.ns1.{LB_DOMAINS}",
						"svc2.ns2.{LB_DOMAINS}",
					},
				},
			},
			expection: false,
		},
	}
	for idx, tc := range testcase {
		ret := NeedUpdate(lb, tc.listeners)
		a.Equal(tc.expection, ret, "Failed on test case No. %d", idx+1)
	}
}
