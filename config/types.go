package config

import "time"

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

type IModeConfig interface {
	GetMode() Mode
}

// TODO use IControllerConfig
type IConfig interface {
	GetNs() string
	GetAlbName() string
	GetDomain() string
	GetPodName() string
	GetNetworkMode() ControllerNetWorkMode
	IsEnableAlb() bool

	ILabelConfig
	IIngressConfig
	ILeaderConfig
	IDebugConfig
	IGatewayConfig
}

type IIngressConfig interface {
	GetDefaultSSLStrategy() string
	GetDefaultSSLSCert() string
	GetIngressHttpPort() int
	GetIngressHttpsPort() int
	GetLabelSourceIngressVer() string
	GetLabelSourceIngressRuleIndex() string
	GetLabelSourceIngressPathIndex() string
}

type ILabelConfig interface {
	GetLabelLeader() string
	GetLabelAlbName() string
	GetLabelSourceType() string
}

type LeaderConfig struct {
	LeaseDuration time.Duration
	RenewDeadline time.Duration
	RetryPeriod   time.Duration
}
type ILeaderConfig interface {
	GetLeaderConfig() LeaderConfig
}

type IDebugConfig interface {
	DebugRuleSync() bool
}

type IGatewayConfig interface {
	GetGatewayCfg() GatewayCfg
}
