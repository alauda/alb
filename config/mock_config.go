package config

import (
	"time"

	. "alauda.io/alb2/pkg/config"
)

// type MockConfig ALBRunConfig

func DefaultMock() *Config {
	return &Config{
		ALBRunConfig: ALBRunConfig{
			Name:   "alb-dev",
			Ns:     "cpaas-system",
			Domain: "cpaas.io",
			Controller: ControllerConfig{
				HttpPort:    80,
				HttpsPort:   443,
				NetworkMode: "host",
				MetricsPort: 1936,
				BackLog:     100,
				Flags: ControllerFlags{
					EnableIPV6: true,
				},
			},
			Gateway: GatewayConfig{},
		},
		ExtraConfig: ExtraConfig{
			Leader: LeaderConfig{
				LeaseDuration: time.Second * time.Duration(120),
				RenewDeadline: time.Second * time.Duration(40),
				RetryPeriod:   time.Second * time.Duration(12),
				SkipExit:      true,
			},
		},
		Names: NewNames("cpaas.io"),
	}
}

func UseMock(cfg *Config) {
	if cfg == nil {
		cfg = DefaultMock()
	}
	InTestSetConfig(*cfg)
}
