package ingress

// 在ingress上设置status
// 这里要注意的是
// 1. alb在配置address时,用户可以写多个地址。可能写ipv4,v6和域名，每个地址都是一个status
// 2. 会有多个alb同时处理一个ingress的情况(设置了相同的项目就会这样)
// 3. 会有多个alb设置了相同的访问地址的情况

// 所以当且仅当 status中没有自己的地址，并且自己处理这个ingress时，将自己的地址和端口加入status中
// 因为sentry有可能会还原ingress，所以没办法通过在annotation上加东西来维护状态
// 目前通过在
//   1. alb 项目更新
//   2. ingress class 更新 (改成别的class了)
//   3. alb地址更新
//   4. alb被删除
// 等具体的事件触发时做更新 来避免每次都reconcile,导致出现status一直被删除的情况
// 有可能会因为事件丢失导致无法删除多余的status，但是在大部分情况下应该是正常的。。

import (
	m "alauda.io/alb2/controller/modules"
	"alauda.io/alb2/utils"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/google/go-cmp/cmp"
	"github.com/samber/lo"
	n1 "k8s.io/api/networking/v1"

	alb2v2 "alauda.io/alb2/pkg/apis/alauda/v2beta1"
	cfg "alauda.io/alb2/pkg/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type AlbAddress struct {
	ips   []string
	hosts []string
}

func (a AlbAddress) ToList() []string {
	return append(a.ips, a.hosts...)
}

func listAddress(alb *alb2v2.ALB2) AlbAddress {
	ips, hosts := utils.ParseAddressList(alb.GetAllAddress())
	return AlbAddress{
		ips:   ips,
		hosts: hosts,
	}
}

func (c *Controller) NeedUpdateIngressStatus(alb *m.AlaudaLoadBalancer, ing *n1.Ingress) bool {
	ports := []int32{}
	need := getIngressFtTypes(ing, c.Config)
	if need.NeedHttp() {
		ports = append(ports, int32(c.GetIngressHttpPort()))
	}
	if need.NeedHttps() {
		ports = append(ports, int32(c.GetIngressHttpsPort()))
	}
	return FillupIngressStatus(listAddress(alb.Alb), ports, ing.DeepCopy())
}

func (c *Controller) UpdateIngressStatus(alb *m.AlaudaLoadBalancer, ing *n1.Ingress) error {
	ports := []int32{}
	need := getIngressFtTypes(ing, c.Config)
	if need.NeedHttp() {
		ports = append(ports, int32(c.GetIngressHttpPort()))
	}
	if need.NeedHttps() {
		ports = append(ports, int32(c.GetIngressHttpsPort()))
	}
	update := FillupIngressStatus(listAddress(alb.Alb), ports, ing)
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
			Port:     p,
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

func (c *Controller) onIngressclassChange(ing *n1.Ingress) error {
	c.log.Info("ingressclass change", "newclass", ing.Spec.IngressClassName)
	alb, err := c.getV2Alb()
	if err != nil {
		return err
	}
	return c.RemoveIngressStatusAaddress(listAddress(alb).ToList(), ing)
}

func (c *Controller) getV2Alb() (*alb2v2.ALB2, error) {
	return c.albInformer.Lister().ALB2s(c.GetNs()).Get(c.GetAlbName())
}

func (c *Controller) RemoveIngressStatusAaddress(address []string, ing *n1.Ingress) error {
	l := c.log.WithName("ing-status").WithValues("ing", ing.Name, "ns", ing.Namespace)
	addressMap := map[string]bool{}
	for _, a := range address {
		addressMap[a] = true
	}
	update := false
	status := []n1.IngressLoadBalancerIngress{}
	for _, ins := range ing.Status.LoadBalancer.Ingress {
		if addressMap[ins.Hostname] || addressMap[ins.IP] {
			l.Info("delete ingress status", "address", address, "ing-status", ing.Status.LoadBalancer.Ingress)
			update = true
			continue
		}
		status = append(status, ins)
	}
	if update {
		old := ing.DeepCopy()
		ing.Status.LoadBalancer.Ingress = status
		newing, err := c.kd.UpdateIngressStatus(ing)
		if err != nil {
			c.log.Error(err, "delete ingress status update fail")
			return err
		}
		c.log.Info("update ingress status to remove myself success", "myself", addressMap, "new", status, "old", old.Status.LoadBalancer.Ingress, "diff", cmp.Diff(old.Status, newing.Status))
	}
	return nil
}

func (c *Controller) onAlbDelete(alb *alb2v2.ALB2) {
	l := c.log.WithName("cleanup")
	l.Info("alb deleting")
	ings, err := c.kd.ListAllIngress()
	if err != nil {
		l.Error(err, "list ingress fail")
		return
	}

	addressList := listAddress(alb).ToList()
	for _, ing := range ings {
		_ = c.RemoveIngressStatusAaddress(addressList, ing)
	}
	controllerutil.RemoveFinalizer(alb, cfg.Alb2Finalizer)
	_, err = c.kd.ALBClient.CrdV2beta1().ALB2s(c.GetNs()).Update(c.kd.Ctx, alb, metav1.UpdateOptions{})
	if err != nil {
		l.Error(err, "remove finalizer fail")
	}
	l.Info("remove alb finalizer ok. clean over")
}

func (c *Controller) onAlbChangeUpdateIngressStatus(oldalb, newalb *alb2v2.ALB2) error {
	address := mapset.NewSet(listAddress(newalb).ToList()...)
	oldaddress := mapset.NewSet(listAddress(oldalb).ToList()...)
	project := mapset.NewSet(newalb.Spec.Config.Projects...)
	oldproject := mapset.NewSet(oldalb.Spec.Config.Projects...)
	l := c.log.WithName("alb-change")
	l.Info("change", "address", cmp.Diff(address, oldaddress), "project", cmp.Diff(project, oldproject))
	// 如果没有减少项目或者地址的话，不需要更新ingress的status
	if address.IsSuperset(oldaddress) && project.IsSuperset(oldproject) {
		c.log.Info("project and address not remove", "address", address.Difference(oldaddress), "project", project.Difference(oldproject))
		return nil
	}
	ings, err := c.kd.ListAllIngress()
	if err != nil {
		l.Error(err, "list ingress fail")
		return err
	}
	addressList := append(address.ToSlice(), oldaddress.ToSlice()...)
	// 当是地址变化时，要把旧的地址去掉
	// 当是project变化时，要把当前的address去掉
	// 当alb变化时，所有的ingress都要resync一次，那时会保证需要设置的status都更新上去了，所以这里我们直接把所有的地址都去掉就行了
	for _, ing := range ings {
		_ = c.RemoveIngressStatusAaddress(addressList, ing)
	}
	return nil
}
