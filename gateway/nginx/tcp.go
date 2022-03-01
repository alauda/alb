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

type TcpProtocolTranslate struct {
	drv *driver.KubernetesDriver
	log logr.Logger
}

func NewTcpProtocolTranslate(drv *driver.KubernetesDriver, log logr.Logger) TcpProtocolTranslate {
	return TcpProtocolTranslate{drv: drv, log: log}
}

func (t *TcpProtocolTranslate) TransLate(ls []*Listener, ftMap FtMap) error {
	portMap := make(map[gatewayType.PortNumber][]*TCPRoute)
	{
		for _, l := range ls {
			if l.Protocol != gatewayType.TCPProtocolType {
				continue
			}
			if _, ok := portMap[l.Port]; !ok {
				portMap[l.Port] = []*TCPRoute{}
			}
			for _, r := range l.routes {
				tcproute, ok := r.(*TCPRoute)
				if !ok {
					continue
				}
				portMap[l.Port] = append(portMap[l.Port], tcproute)
			}
		}
		t.log.Info("translate tcp protocol rule", "ls-len", len(ls), "tcp-ports-len", len(portMap))
	}

	for port, rs := range portMap {
		if len(rs) == 0 {
			t.log.Info("could not found vaild route", "error", true)
			return nil
		}
		if len(rs) > 1 {
			t.log.Info("tcp has more than one route", "port", port)
			return nil
		}
		route := rs[0]
		t.log.Info("generated rule ", "port", port, "route", route)

		ft := &Frontend{
			Port:     int(port),
			Protocol: albType.FtProtocolTCP,
		}
		// TODO we donot support multiple tcp rules
		if len(route.Spec.Rules) != 1 {
			t.log.Error(fmt.Errorf("we do not support multiple tcp rules"), "port", port, "route", GetObjectKey(route))
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
