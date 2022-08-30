package config

import "time"

// TODO split it into more specific interface,like basic/ingress/gateway/label etc
type IConfig interface {
	GetNs() string
	GetAlbName() string
	GetDomain() string
	GetPodName() string
	ILabelConfig
	IIngressConfig
	ILeaderConfig
	IDebugConfig
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
