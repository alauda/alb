package assert

import (
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/samber/lo"
	n1 "k8s.io/api/networking/v1"
)

type IngressAssert struct {
	ing *n1.Ingress
}

func NewIngressAssert(ing *n1.Ingress) *IngressAssert {
	return &IngressAssert{
		ing: ing,
	}
}

func (i *IngressAssert) HasLoadBalancerIP(ip string) bool {
	for _, lb := range i.ing.Status.LoadBalancer.Ingress {
		if lb.IP == ip {
			return true
		}
	}
	return false
}

func (i *IngressAssert) HasLoadBalancerHost(host string) bool {
	for _, lb := range i.ing.Status.LoadBalancer.Ingress {
		if lb.Hostname == host {
			return true
		}
	}
	return false
}

func (i *IngressAssert) HasLoadBalancerHostAndPort(host string, port []int32) bool {
	pmap := mapset.NewSet(port...)
	for _, lb := range i.ing.Status.LoadBalancer.Ingress {
		if lb.Hostname != host {
			continue
		}
		ports := lo.Map(lb.Ports, func(p n1.IngressPortStatus, _ int) int32 {
			return p.Port
		})
		if pmap.Equal(mapset.NewSet(ports...)) {
			return true
		}
	}
	return false
}
