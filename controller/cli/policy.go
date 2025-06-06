package cli

import (
	"fmt"
	"time"

	. "alauda.io/alb2/controller/types"
	"alauda.io/alb2/driver"
	albv1 "alauda.io/alb2/pkg/apis/alauda/v1"
	cus "alauda.io/alb2/pkg/controller/extctl"
	"alauda.io/alb2/pkg/utils"
	pm "alauda.io/alb2/pkg/utils/metrics"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type PolicyCli struct {
	drv *driver.KubernetesDriver
	log logr.Logger
	cus cus.ExtCtl
	opt PolicyCliOpt
}
type PolicyCliOpt struct {
	MetricsPort    int
	DefaultSSLCert string
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

	// set metrics port cert
	extra_secret_map := make(map[string]client.ObjectKey)
	metrics_port_str := fmt.Sprintf("%d", p.opt.MetricsPort)
	if p.opt.MetricsPort != 0 && p.opt.DefaultSSLCert != "" {
		cert_key, err := utils.ParseStringToObjectKey(p.opt.DefaultSSLCert)
		if err == nil {
			extra_secret_map[metrics_port_str] = cert_key
		}
	}
	certificateMap := getCertMap(alb, p.drv, extra_secret_map)
	if p.opt.MetricsPort != 0 && !utils.KeyExists(certificateMap, metrics_port_str) {
		certificateMap[metrics_port_str] = genMetricsCert()
	}

	s = time.Now()
	backendGroup := pickAllBackendGroup(alb)
	pm.Write("gen-policy/pick", float64(time.Since(s_other).Milliseconds()))

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
