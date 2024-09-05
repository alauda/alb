package modules

import (
	alb2v1 "alauda.io/alb2/pkg/apis/alauda/v1"
	albv2 "alauda.io/alb2/pkg/apis/alauda/v2beta1"
	"golang.org/x/exp/maps"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// a tree like struct to load all alb route related resource such as alb/ft/rule
type AlaudaLoadBalancer struct {
	Alb       *albv2.ALB2
	Frontends []*Frontend
}

type Frontend struct {
	*alb2v1.Frontend
	Rules []*Rule
	LB    *AlaudaLoadBalancer
}

type Rule struct {
	*alb2v1.Rule
	FT *Frontend
}

func (r *Rule) GetFtConfig() *alb2v1.FTConfig {
	if r == nil {
		return nil
	}
	if r.FT == nil {
		return nil
	}
	return r.FT.Spec.Config
}

func (r *Rule) GetAlbConfig() *albv2.ExternalAlbConfig {
	if r == nil {
		return nil
	}
	if r.FT == nil {
		return nil
	}
	if r.FT.LB == nil {
		return nil
	}
	if r.FT.LB.Alb == nil {
		return nil
	}
	return r.FT.LB.Alb.Spec.Config
}

func (alb *AlaudaLoadBalancer) GetAlbKey() client.ObjectKey {
	return client.ObjectKey{
		Namespace: alb.Alb.Namespace,
		Name:      alb.Alb.Name,
	}
}

func (alb *AlaudaLoadBalancer) FindIngressFt(port int, protocol alb2v1.FtProtocol) *Frontend {
	for _, f := range alb.Frontends {
		if int(f.Spec.Port) == port && f.Spec.Protocol == protocol {
			return f
		}
	}
	return nil
}

func (alb *AlaudaLoadBalancer) FindHandledIngressKey() []client.ObjectKey {
	keyM := map[client.ObjectKey]bool{}
	for _, f := range alb.Frontends {
		fsource := f.Spec.Source
		if fsource != nil && fsource.Type == TypeIngress {
			key := client.ObjectKey{Namespace: fsource.Namespace, Name: fsource.Name}
			keyM[key] = true
		}
		for _, r := range f.Rules {
			rsource := r.Spec.Source
			if rsource != nil && rsource.Type == TypeIngress {
				key := client.ObjectKey{Namespace: rsource.Namespace, Name: rsource.Name}
				keyM[key] = true
			}
		}
	}
	return maps.Keys(keyM)
}

func (alb *AlaudaLoadBalancer) FindIngressFtRaw(port int, protocol alb2v1.FtProtocol) *alb2v1.Frontend {
	f := alb.FindIngressFt(port, protocol)
	if f != nil {
		return f.Frontend
	}
	return nil
}

func (ft *Frontend) IsTcpOrUdp() bool {
	if ft.Spec.Protocol == alb2v1.FtProtocolTCP {
		return true
	}
	if ft.Spec.Protocol == alb2v1.FtProtocolUDP {
		return true
	}
	return false
}

func (f *Frontend) IsCreateByThisIngress(ns, name string) bool {
	ft := f.Spec
	return ft.Source != nil &&
		ft.Source.Type == TypeIngress &&
		ft.Source.Namespace == ns &&
		ft.Source.Name == name
}

func (ft *Frontend) IsHttpOrHttps() bool {
	return ft.IsHttp() || ft.IsHttpS()
}

func (ft *Frontend) IsHttp() bool {
	return ft.Spec.Protocol == alb2v1.FtProtocolHTTP
}

func (ft *Frontend) IsHttpS() bool {
	return ft.Spec.Protocol == alb2v1.FtProtocolHTTPS
}

func (ft *Frontend) IsgRPC() bool {
	return ft.Spec.Protocol == alb2v1.FtProtocolgRPC
}

func (ft *Frontend) SamePort(other int) bool {
	return int(ft.Spec.Port) == other
}

func (ft *Frontend) FindIngressRule(key client.ObjectKey) []*Rule {
	if ft == nil {
		return nil
	}
	ret := []*Rule{}
	for _, r := range ft.Rules {
		source := r.Spec.Source
		if source == nil {
			continue
		}
		if source.Type != TypeIngress {
			continue
		}
		if source.Name == key.Name && source.Namespace == key.Namespace {
			ret = append(ret, r)
		}
	}
	return ret
}

func (ft *Frontend) FindIngressRuleRaw(key client.ObjectKey) []*alb2v1.Rule {
	ret := []*alb2v1.Rule{}
	for _, r := range ft.FindIngressRule(key) {
		ret = append(ret, r.Rule)
	}
	return ret
}

func (ft *Frontend) Raw() *alb2v1.Frontend {
	if ft == nil {
		return nil
	}
	return ft.Frontend
}

func (ft *Frontend) HaveDefaultBackend() bool {
	return ft.Spec.BackendProtocol != "" && len(ft.Spec.ServiceGroup.Services) != 0
}

func SetDefaultBackend(ft *alb2v1.Frontend, protocol string, svc *alb2v1.ServiceGroup) {
	ft.Spec.BackendProtocol = protocol
	ft.Spec.ServiceGroup = svc
}

func SetSource(ft *alb2v1.Frontend, ing *networkingv1.Ingress) {
	ft.Spec.Source = &alb2v1.Source{
		Name:      ing.Name,
		Namespace: ing.Namespace,
		Type:      TypeIngress,
	}
}

func (rule *Rule) IsCreateByThisIngress(ns, name string) bool {
	source := rule.Spec.Source
	return source != nil &&
		source.Type == TypeIngress &&
		source.Namespace == ns &&
		source.Name == name
}

func (r *Rule) Key() client.ObjectKey {
	return client.ObjectKey{
		Namespace: r.FT.Namespace,
		Name:      r.Name,
	}
}

func RuleKey(r *alb2v1.Rule) client.ObjectKey {
	return client.ObjectKey{
		Namespace: r.Namespace,
		Name:      r.Name,
	}
}

func FtProtocolToServiceProtocol(protocol alb2v1.FtProtocol) corev1.Protocol {
	if protocol == alb2v1.FtProtocolUDP {
		return corev1.ProtocolUDP
	}
	return corev1.ProtocolTCP
}
