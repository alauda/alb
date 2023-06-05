package ingress

// 在ingress上设置status
// 这里要注意的是
// 1. alb在配置address时,用户可以写多个地址。可能写ipv4,v6和域名，每个地址都是一个status
// 2. 会有多个alb同时处理一个ingress的情况
// 3. 会有多个alb设置了相同的访问地址的情况

// 所以这里暂时当且仅当 status中没有自己的地址，并且自己处理这个ingress时，将自己的地址和端口加入status中
// 不从status中删除地址

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

type AlbAddress struct {
	ips   []string
	hosts []string
}

func (c *Controller) NeedUpdateIngressStatus(alb *m.AlaudaLoadBalancer, ing *n1.Ingress) bool {
	ports := []int32{}
	need := getIngressFtTypes(ing, c)
	if need.NeedHttp() {
		ports = append(ports, int32(c.GetIngressHttpPort()))
	}
	if need.NeedHttps() {
		ports = append(ports, int32(c.GetIngressHttpsPort()))
	}
	return FillupIngressStatus(listAddress(alb), ports, ing.DeepCopy())
}

func listAddress(alb *m.AlaudaLoadBalancer) AlbAddress {
	ipsfromSpec, hostFromSpec := parseAddress(alb.Spec.Address)
	ips := ipsfromSpec
	ips = append(ips, alb.Status.Detail.AddressStatus.Ipv4...)
	ips = append(ips, alb.Status.Detail.AddressStatus.Ipv6...)
	hosts := hostFromSpec
	return AlbAddress{
		ips:   ips,
		hosts: hosts,
	}
}

func (c *Controller) UpdateIngressStatus(alb *m.AlaudaLoadBalancer, ing *n1.Ingress) error {

	ports := []int32{}
	need := getIngressFtTypes(ing, c)
	if need.NeedHttp() {
		ports = append(ports, int32(c.GetIngressHttpPort()))
	}
	if need.NeedHttps() {
		ports = append(ports, int32(c.GetIngressHttpsPort()))
	}
	update := FillupIngressStatus(listAddress(alb), ports, ing)
	if !update {
		return nil
	}
	newing, err := c.kd.UpdateIngressStatus(ing)
	if err != nil {
		return err
	}
	c.log.Info("update ingress status", "diff", cmp.Diff(ing, newing))
	return nil
}

func FillupIngressStatus(address AlbAddress, ports []int32, ing *n1.Ingress) bool {
	ips := address.ips
	hosts := address.hosts
	update := false
	for _, ip := range ips {
		if FillupIngressStatusAddressAndPort(ing, ip, "", ports) {
			update = true
		}
	}
	for _, host := range hosts {
		if FillupIngressStatusAddressAndPort(ing, "", host, ports) {
			update = true
		}
	}
	return update
}

// set ip or host and ports to ingress status. return true if ingress status changed
func FillupIngressStatusAddressAndPort(ing *n1.Ingress, ip string, host string, ports []int32) bool {
	portSet := mapset.NewSet(ports...)
	var status *n1.IngressLoadBalancerIngress = nil
	for i, s := range ing.Status.LoadBalancer.Ingress {
		if (s.IP == ip && ip != "") || (s.Hostname == host && host != "") {
			status = &ing.Status.LoadBalancer.Ingress[i]
		}
	}

	// 补全已有的status的port
	if status != nil {
		curPortSet := mapset.NewSet(lo.Map(status.Ports, func(p n1.IngressPortStatus, _ int) int32 {
			return p.Port
		})...)
		missPort := portSet.Difference(curPortSet).ToSlice()
		for _, p := range missPort {
			status.Ports = append(status.Ports, n1.IngressPortStatus{
				Port:     p,
				Protocol: "TCP",
			})
		}
		return len(missPort) > 0
	}

	// 添加新的status
	ingPorts := []n1.IngressPortStatus{}
	for _, p := range ports {
		ingPorts = append(ingPorts, n1.IngressPortStatus{
			Port:     int32(p),
			Protocol: "TCP",
		})
	}
	ing.Status.LoadBalancer.Ingress = append(ing.Status.LoadBalancer.Ingress, n1.IngressLoadBalancerIngress{
		IP:       ip,
		Hostname: host,
		Ports:    ingPorts,
	})
	return true
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
