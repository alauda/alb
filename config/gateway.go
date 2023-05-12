package config

import (
	"fmt"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type GatewayMode string

const (
	Gateway      GatewayMode = "gateway"
	GatewayClass GatewayMode = "gatewayclass"
)

func getGatewayName() (ns string, name string, err error) {
	gname := Get("GATEWAY_NAME")
	names := strings.Split(gname, "/")
	if len(names) != 2 {
		return "", "", fmt.Errorf("invalid name %s", names)
	}
	return names[0], names[1], nil
}

type GatewayCfg struct {
	Enable          bool
	Mode            GatewayMode
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
	enable := GetBool("GATEWAY_ENABLE")
	if !enable {
		return GatewayCfg{
			Enable: false,
		}
	}

	modeStr := Get("GATEWAY_MODE")
	mode := Gateway
	if modeStr == string(Gateway) {
		mode = Gateway
	}
	if modeStr == string(GatewayClass) {
		mode = GatewayClass
	}
	var sel GatewaySelector
	if enable && mode == Gateway {
		ns, name, err := getGatewayName()
		if err != nil {
			panic(err)
		}
		sel = GatewaySelector{
			GatewayName: &client.ObjectKey{Name: name, Namespace: ns},
		}
	}
	if enable && mode == GatewayClass {
		name := Get("NAME")
		gatewayName := Get("GATEWAY_NAME")
		if gatewayName != "" {
			name = gatewayName
		}
		sel = GatewaySelector{
			GatewayClass: &name,
		}
	}
	return GatewayCfg{
		Enable:          enable,
		Mode:            mode,
		GatewaySelector: sel,
		ReservedPort:    c.GetMetricsPort(),
	}
}
