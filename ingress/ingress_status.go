package ingress

// 在ingress上设置status
// 这里要注意的是
// 1. alb在配置address时,用户可以写多个地址。可能写ipv4,v6和域名，每个地址都是一个status
// 2. 会有多个alb同时处理一个ingress的情况，所以更新ingress的status时,我们要在annotation上写下这个alb的address.
// 3. 如果有多个alb设置了相同的地址。在其中一个alb不在处理这个ingress的时候。可能会导致误删。导致其他alb的status被清理掉,
//       所以在我们维护状态的时候是以annotation为标准。如果annotation没有我们，即使status有我们，也不要清理。

import (
	"net/url"
	"strings"

	m "alauda.io/alb2/modules"
	"alauda.io/alb2/utils"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/google/go-cmp/cmp"
	"github.com/samber/lo"
	n1 "k8s.io/api/networking/v1"
)

func (c *Controller) NeedUpdateIngressStatus(alb *m.AlaudaLoadBalancer, ing *n1.Ingress) bool {
	key := c.GetAnnotationIngressAddress()
	val := ing.Annotations[key]
	if val != alb.Spec.Address {
		return true
	}
	if !c.ingressStatusHasPort(ing) {
		return true
	}
	return false
}

func (c *Controller) ingressStatusHasPort(ing *n1.Ingress) bool {
	need := getIngressFtTypes(ing, c)
	needPort := mapset.NewSet[int32]()
	if need.NeedHttp() {
		needPort.Add(int32(c.GetIngressHttpPort()))
	}
	if need.NeedHttps() {
		needPort.Add(int32(c.GetIngressHttpsPort()))
	}

	address := ing.Annotations[c.GetAnnotationIngressAddress()]
	ips, hosts := parseAddress(address)
	ipset := mapset.NewSet(ips...)
	hostset := mapset.NewSet(hosts...)

	for _, status := range ing.Status.LoadBalancer.Ingress {
		portset := mapset.NewSet(lo.Map(status.Ports, func(p n1.IngressPortStatus, _ int) int32 {
			return p.Port
		})...)
		if ipset.Contains(status.IP) && !portset.Equal(needPort) {
			return false
		}
		if hostset.Contains(status.Hostname) && !portset.Equal(needPort) {
			return false
		}
	}
	return true
}

func (c *Controller) UpdateIngressStatus(alb *m.AlaudaLoadBalancer, ing *n1.Ingress) error {
	old := ing.DeepCopy()
	c.removeOurIngressStatus(alb.Name, ing)
	key := c.GetAnnotationIngressAddress()
	val := alb.Spec.Address
	ing.Annotations[key] = val
	need := getIngressFtTypes(ing, c)
	status := genStatusFromAddress(val, need, int32(c.GetIngressHttpPort()), int32(c.GetIngressHttpsPort()))
	ing.Status.LoadBalancer.Ingress = append(ing.Status.LoadBalancer.Ingress, status...)
	ing, err := c.kd.UpdateIngressAndStatus(ing)
	if err != nil {
		return err
	}
	c.log.Info("update ingress status success", "diff", cmp.Diff(old, ing))
	return nil
}

// clean up ingress status if exist. input ing will NOT change.
func (c *Controller) CleanUpIngressStatus(alb *m.AlaudaLoadBalancer, ing *n1.Ingress) error {
	// ingress could not found.
	if ing == nil {
		return nil
	}
	if c.hasOurStatus(alb.Name, ing) {
		c.removeOurIngressStatus(alb.Name, ing)
		old := ing.DeepCopy()
		ing, err := c.kd.UpdateIngressAndStatus(ing)
		c.log.Info("cleanup ingress status success", "diff", cmp.Diff(old, ing))
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *Controller) hasOurStatus(name string, ing *n1.Ingress) bool {
	key := c.GetAnnotationIngressAddress()
	return ing.Annotations[key] != ""
}

// update the input ingress, remove annotation and status which set by us
func (c *Controller) removeOurIngressStatus(name string, ing *n1.Ingress) {
	key := c.GetAnnotationIngressAddress()
	val := ing.Annotations[key]
	delete(ing.Annotations, key)
	address := mapset.NewSet(splitAddress(val)...)
	status := ing.Status.LoadBalancer.Ingress
	newStatus := []n1.IngressLoadBalancerIngress{}
	for _, s := range status {
		key := ingressStatuskey(s)
		if !address.Contains(key) {
			newStatus = append(newStatus, s)
		}
	}
	ing.Status.LoadBalancer.Ingress = newStatus
}

func addressIs(address string) (ipv4 bool, ipv6 bool, domain bool, err error) {
	if utils.IsValidIPv4(address) {
		return true, false, false, nil
	}
	if utils.IsValidIPv6(address) {
		return false, true, false, nil
	}
	_, err = url.Parse(address)
	if err != nil {
		return false, false, false, err
	}
	return false, false, true, nil
}

func splitAddress(address string) []string {
	ip, host := parseAddress(address)
	return append(ip, host...)
}

func parseAddress(address string) (ip []string, host []string) {
	addrs := strings.Split(address, ",")
	ip = []string{}
	host = []string{}
	for _, addr := range addrs {
		addr = strings.TrimSpace(addr)
		if addr == "" {
			continue
		}
		v4, v6, hostname, err := addressIs(addr)
		if err != nil {
			continue
		}
		if v4 || v6 {
			ip = append(ip, addr)
		}
		if hostname {
			host = append(host, addr)
		}
	}
	return ip, host
}

func genStatusFromAddress(address string, need Need, http int32, https int32) []n1.IngressLoadBalancerIngress {
	ips, hosts := parseAddress(address)
	out := []n1.IngressLoadBalancerIngress{}
	ports := []n1.IngressPortStatus{}
	if need.NeedHttp() {
		ports = append(ports, n1.IngressPortStatus{
			Protocol: "TCP", // only could be TCP/UDP/SCTP
			Port:     http,
		})
	}
	if need.NeedHttps() {
		ports = append(ports, n1.IngressPortStatus{
			Protocol: "TCP",
			Port:     https,
		})
	}
	for _, ip := range ips {
		out = append(out, n1.IngressLoadBalancerIngress{
			IP:    ip,
			Ports: ports,
		})
	}
	for _, hosts := range hosts {
		out = append(out, n1.IngressLoadBalancerIngress{
			Hostname: hosts,
			Ports:    ports,
		})
	}
	return out
}

func ingressStatuskey(ing n1.IngressLoadBalancerIngress) string {
	if ing.IP != "" {
		return ing.IP
	}
	if ing.Hostname != "" {
		return ing.Hostname
	}
	return ""
}
