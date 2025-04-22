package cli

import (
	"fmt"
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
	all := time.Now()
	defer func() {
		pm.Write("gen-policy-all", float64(time.Since(all).Milliseconds()))
	}()

	s := time.Now()
	certificateMap := getCertMap(alb, p.drv)
	pm.Write("gen-policy/cert", float64(time.Since(s).Milliseconds()))

	s = time.Now()
	p.setMetricsPortCert(certificateMap)
	pm.Write("gen-policy/set-metrics-port-cert", float64(time.Since(s).Milliseconds()))

	s = time.Now()
	backendGroup := pickAllBackendGroup(alb)
	pm.Write("gen-policy/pick-backend-group", float64(time.Since(s).Milliseconds()))

	ngxPolicy := NgxPolicy{
		CertificateMap: certificateMap,
		Http:           HttpPolicy{Tcp: make(map[albv1.PortNumber]Policies)},
		SharedConfig:   SharedExtPolicyConfig{},
		Stream:         StreamPolicy{Tcp: make(map[albv1.PortNumber]Policies), Udp: make(map[albv1.PortNumber]Policies)},
		BackendGroup:   backendGroup,
	}

	s = time.Now()
	for _, ft := range alb.Frontends {
		if ft.Conflict {
			continue
		}
		sf := time.Now()
		if ft.IsStreamMode() {
			p.initStreamModeFt(ft, &ngxPolicy)
		} else {
			p.initHttpModeFt(ft, &ngxPolicy, alb.Refs)
		}
		key := fmt.Sprintf("gen-policy/init-ft-%d-%s", ft.Port, ft.Protocol)
		pm.Write(key, float64(time.Since(sf).Milliseconds()))
	}

	pm.Write("gen-policy/init-ft", float64(time.Since(s).Milliseconds()))

	s = time.Now()
	p.cus.ResolvePolicies(alb, &ngxPolicy)
	pm.Write("gen-policy/resolve-policies", float64(time.Since(s).Milliseconds()))

	return ngxPolicy
}
