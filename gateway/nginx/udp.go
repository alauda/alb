package nginx

import (
	"fmt"

	"alauda.io/alb2/driver"
	"github.com/go-logr/logr"

	. "alauda.io/alb2/controller/types"
	. "alauda.io/alb2/gateway"

	albType "alauda.io/alb2/pkg/apis/alauda/v1"
	gatewayType "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

type UdpProtocolTranslate struct {
	drv *driver.KubernetesDriver
	log logr.Logger
}

func NewUdpProtocolTranslate(drv *driver.KubernetesDriver, log logr.Logger) UdpProtocolTranslate {
	return UdpProtocolTranslate{drv: drv, log: log}
}

func (t *UdpProtocolTranslate) TransLate(ls []*Listener, ftMap FtMap) error {
	portMap := make(map[gatewayType.PortNumber][]*UDPRoute)
	{
		for _, l := range ls {
			if l.Protocol != gatewayType.UDPProtocolType {
				continue
			}
			if _, ok := portMap[l.Port]; !ok {
				portMap[l.Port] = []*UDPRoute{}
			}
			for _, r := range l.routes {
				udproute, ok := r.(*UDPRoute)
				if !ok {
					continue
				}
				portMap[l.Port] = append(portMap[l.Port], udproute)
			}
		}
		t.log.Info("translate udp protocol rule", "ls-len", len(ls), "udp-ports-len", len(portMap))
	}

	for port, rs := range portMap {
		if len(rs) == 0 {
			t.log.Info("could not found vaild route", "error", true)
			return nil
		}
		if len(rs) > 1 {
			t.log.Info("udp has more than one route", "port", port)
			return nil
		}
		route := rs[0]
		t.log.Info("generated rule ", "port", port, "route", route)

		ft := &Frontend{
			Port:     int(port),
			Protocol: albType.FtProtocolUDP,
		}
		// TODO we donot support multiple udp rules
		if len(route.Spec.Rules) != 1 {
			t.log.Error(fmt.Errorf("we do not support multiple udp rules"), "port", port, "route", GetObjectKey(route))
			return nil
		}
		svcs, err := backendRefsToService(route.Spec.Rules[0].BackendRefs)
		if err != nil {
			return nil
		}
		ft.Services = svcs
		ft.BackendGroup = &BackendGroup{
			Name: fmt.Sprintf("%v-%v-%v", port, route.Namespace, route.Name),
		}

		ftMap.SetFt(string(ft.Protocol), ft.Port, ft)
	}

	return nil
}
