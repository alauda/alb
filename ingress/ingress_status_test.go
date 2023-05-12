package ingress

import (
	"testing"

	"alauda.io/alb2/config"
	. "alauda.io/alb2/utils"
	"github.com/stretchr/testify/assert"
	n1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
		ip, host := parseAddress(c.address)
		assert.Equal(t, c.ip, ip)
		assert.Equal(t, c.hostname, host)
	}
}

func TestRemoveIngressStatus(t *testing.T) {
	cfg := config.DefaultMock()
	key := cfg.GetAnnotationIngressAddress()
	ing := &n1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				key: "127.0.0.1,2004::192:168:128:191,alauda.com",
			},
		},
		Status: n1.IngressStatus{
			LoadBalancer: n1.IngressLoadBalancerStatus{
				Ingress: []n1.IngressLoadBalancerIngress{
					{
						IP: "127.0.0.1",
					},
					{
						IP: "127.0.0.2",
					},
					{
						IP: "2004::192:168:128:191",
					},
					{
						Hostname: "alauda.com",
					},
				},
			},
		},
	}
	expect := &n1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{},
		},
		Status: n1.IngressStatus{
			LoadBalancer: n1.IngressLoadBalancerStatus{
				Ingress: []n1.IngressLoadBalancerIngress{
					{
						IP: "127.0.0.2",
					},
				},
			},
		},
	}
	c := &Controller{
		IConfig: cfg,
	}
	c.removeOurIngressStatus(c.GetAlbName(), ing)
	t.Logf("cur\n%v\nexp\n%v\n", PrettyJson(ing), PrettyJson(expect))
	assert.Equal(t, *ing, *expect)

}
