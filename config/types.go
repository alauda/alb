package config

type Mode string

var ModeKey = "MODE"

const (
	Controller Mode = "controller"
	Operator   Mode = "operator"
)

type ControllerNetWorkMode string

var NetworkModeKey = "NETWORK_MODE"

const (
	Host      ControllerNetWorkMode = "host"
	Container ControllerNetWorkMode = "container"
)

type NginxCfg struct {
	NginxTemplatePath string
	NewConfigPath     string
	OldConfigPath     string
	NewPolicyPath     string
	EnablePrometheus  bool
	EnableHttp2       bool
	EnableGzip        bool
	BackLog           int
	EnableIpv6        bool
	TweakDir          string
}
