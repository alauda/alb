package types

import (
	"fmt"

	albv1 "alauda.io/alb2/pkg/apis/alauda/v1"
	v1 "alauda.io/alb2/pkg/apis/alauda/v1"
	otelt "alauda.io/alb2/pkg/controller/ext/otel/types"
)

func (p Policy) GetOtel() *otelt.OtelConf {
	return p.Config.Otel
}

func (p *NgxPolicy) GetBackendGroup(name string) *BackendGroup {
	for _, be := range p.BackendGroup {
		if be.Name == name {
			return be
		}
	}
	return nil
}

func (p *HttpPolicy) GetPoliciesByPort(port int) Policies {
	return p.Tcp[albv1.PortNumber(port)]
}

func (p Policies) Len() int { return len(p) }

func (p Policies) Less(i, j int) bool {
	// raw priority is set by user it should be [1,10]. the smaller the number, the higher the ranking
	if p[i].Priority != p[j].Priority {
		return p[i].Priority < p[j].Priority
	}
	// priority is calculated by the "complex" of this policy. the bigger the number, the higher the ranking
	if p[i].ComplexPriority != p[j].ComplexPriority {
		return p[i].ComplexPriority > p[j].ComplexPriority
	}
	if p[i].InternalDSLLen != p[j].InternalDSLLen {
		return p[i].InternalDSLLen > p[j].InternalDSLLen
	}
	return p[i].Rule < p[j].Rule
}

func (p Policies) Swap(i, j int) { p[i], p[j] = p[j], p[i] }

func (rl InternalRule) AllowNoAddr() bool {
	return rl.Config.Redirect != nil
}

func (rl InternalRule) GetRawPriority() int {
	return rl.Priority
}

func (rl InternalRule) GetPriority() int {
	return rl.DSLX.Priority()
}

type RuleList []*InternalRule

type BackendGroups []*BackendGroup

func (bgs BackendGroups) Len() int {
	return len(bgs)
}

func (bgs BackendGroups) Swap(i, j int) {
	bgs[i], bgs[j] = bgs[j], bgs[i]
}

func (bgs BackendGroups) Less(i, j int) bool {
	return bgs[i].Name > bgs[j].Name
}

func (bg BackendGroup) Eq(other BackendGroup) bool {
	// bg equal other
	return bg.Name == other.Name &&
		bg.Mode == other.Mode &&
		bg.SessionAffinityAttribute == other.SessionAffinityAttribute &&
		bg.SessionAffinityPolicy == other.SessionAffinityPolicy &&
		bg.Backends.Eq(other.Backends)
}

func FtProtocolToBackendMode(protocol v1.FtProtocol) string {
	switch protocol {
	case v1.FtProtocolTCP:
		return ModeTCP
	case v1.FtProtocolUDP:
		return ModeUDP
	case v1.FtProtocolHTTP:
		return ModeHTTP
	case v1.FtProtocolHTTPS:
		return ModeHTTP
	case v1.FtProtocolgRPC:
		return ModegRPC
	}
	return ""
}

func (ft *Frontend) String() string {
	return fmt.Sprintf("%s-%d-%s", ft.AlbName, ft.Port, ft.Protocol)
}

func (ft *Frontend) IsTcpBaseProtocol() bool {
	return ft.Protocol == v1.FtProtocolHTTP ||
		ft.Protocol == v1.FtProtocolHTTPS ||
		ft.Protocol == v1.FtProtocolTCP
}

func (ft *Frontend) IsStreamMode() bool {
	return ft.Protocol == v1.FtProtocolTCP || ft.Protocol == v1.FtProtocolUDP
}

func (ft *Frontend) IsHttpMode() bool {
	return ft.Protocol == v1.FtProtocolHTTP || ft.Protocol == v1.FtProtocolHTTPS
}

func (ft *Frontend) IsGRPCMode() bool {
	return ft.Protocol == v1.FtProtocolgRPC
}

func (ft *Frontend) IsValidProtocol() bool {
	return ft.Protocol == v1.FtProtocolHTTP ||
		ft.Protocol == v1.FtProtocolHTTPS ||
		ft.Protocol == v1.FtProtocolTCP ||
		ft.Protocol == v1.FtProtocolUDP ||
		ft.Protocol == v1.FtProtocolgRPC
}

func (b *Backend) Eq(other *Backend) bool {
	return b.Address == other.Address && b.Port == other.Port && b.Weight == other.Weight
}

func (b Backend) String() string {
	return fmt.Sprintf("%v-%v-%v", b.Address, b.Port, b.Weight)
}

func (bs Backends) Len() int {
	return len(bs)
}

func (bs Backends) Less(i, j int) bool {
	return bs[i].String() < bs[j].String()
}

func (bs Backends) Swap(i, j int) {
	bs[i], bs[j] = bs[j], bs[i]
}

func (bs Backends) Eq(other Backends) bool {
	if len(bs) != len(other) {
		return false
	}
	for i := range bs {
		if !bs[i].Eq(other[i]) {
			return false
		}
	}
	return true
}

func (r RewriteResponseConfig) IsEmpty() bool {
	return len(r.Headers) == 0
}
