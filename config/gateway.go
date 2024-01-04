package config

import (
	"fmt"

	a2t "alauda.io/alb2/pkg/apis/alauda/v2beta1"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type GatewayCfg struct {
	Enable          bool
	Mode            a2t.GatewayMode
	ReservedPort    int             // listener could use this port, but route should not attach to it.
	GatewaySelector GatewaySelector // if im in gateway mode, which gateway should i watch?
}

func (g GatewayCfg) String() string {
	return fmt.Sprintf("mode %s sel %s", g.Mode, g.GatewaySelector)
}

type GatewaySelector struct {
	GatewayName  *client.ObjectKey
	GatewayClass *string
}

func (g GatewaySelector) String() string {
	if g.GatewayClass != nil {
		return "class " + *g.GatewayClass
	}
	return "gateway " + g.GatewayName.String()
}

func (c *Config) GetGatewayCfg() GatewayCfg {
	enable := c.Gateway.Enable
	if !enable {
		return GatewayCfg{
			Enable: false,
		}
	}

	mode := c.Gateway.Mode
	var sel GatewaySelector
	if enable && mode == a2t.GatewayModeStandAlone {
		ns := c.Gateway.StandAlone.GatewayNS
		name := c.Gateway.StandAlone.GatewayName
		sel = GatewaySelector{
			GatewayName: &client.ObjectKey{Name: name, Namespace: ns},
		}
	}
	if enable && mode == a2t.GatewayModeShared {
		sel = GatewaySelector{
			GatewayClass: &c.Gateway.Shared.GatewayClassName,
		}
	}
	return GatewayCfg{
		Enable:          enable,
		Mode:            mode,
		GatewaySelector: sel,
		ReservedPort:    c.GetMetricsPort(),
	}
}
