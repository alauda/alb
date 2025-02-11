package cli

import (
	"time"

	. "alauda.io/alb2/controller/types"
	"alauda.io/alb2/driver"
	albv1 "alauda.io/alb2/pkg/apis/alauda/v1"
	cus "alauda.io/alb2/pkg/controller/extctl"
	pm "alauda.io/alb2/pkg/utils/metrics"
	"github.com/go-logr/logr"
)

type PolicyCli struct {
	drv *driver.KubernetesDriver
	log logr.Logger
	cus cus.ExtCtl
	opt PolicyCliOpt
}
type PolicyCliOpt struct {
	MetricsPort int
}

func NewPolicyCli(drv *driver.KubernetesDriver, log logr.Logger, opt PolicyCliOpt) PolicyCli {
	return PolicyCli{
		drv: drv,
		log: log,
		opt: opt,
		cus: cus.NewExtensionCtl(cus.ExtCtlCfgOpt{Log: log}),
	}
}

// fetch cert and backend info that lb config need, constructs a "dynamic config" used by openresty.
func (p *PolicyCli) GenerateAlbPolicy(alb *LoadBalancer) NgxPolicy {
	s := time.Now()
	defer func() {
		pm.Write("gen-policy", float64(time.Since(s).Milliseconds()))
	}()

	s_other := time.Now()
	certificateMap := getCertMap(alb, p.drv)

	p.setMetricsPortCert(certificateMap)
	backendGroup := pickAllBackendGroup(alb)
	pm.Write("gen-policy/pick", float64(time.Since(s_other).Milliseconds()))

	ngxPolicy := NgxPolicy{
		CertificateMap: certificateMap,
		Http:           HttpPolicy{Tcp: make(map[albv1.PortNumber]Policies)},
		SharedConfig:   SharedExtPolicyConfig{},
		Stream:         StreamPolicy{Tcp: make(map[albv1.PortNumber]Policies), Udp: make(map[albv1.PortNumber]Policies)},
		BackendGroup:   backendGroup,
	}

	sf := time.Now()
	for _, ft := range alb.Frontends {
		if ft.Conflict {
			continue
		}
		if ft.IsStreamMode() {
			p.initStreamModeFt(ft, &ngxPolicy)
		} else {
			p.initHttpModeFt(ft, &ngxPolicy, alb.Refs)
		}
	}

	pm.Write("gen-policy/init-ft", float64(time.Since(sf).Milliseconds()))
	p.cus.ResolvePolicies(alb, &ngxPolicy)
	return ngxPolicy
}
