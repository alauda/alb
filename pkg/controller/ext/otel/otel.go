package otel

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strings"

	"alauda.io/alb2/config"
	m "alauda.io/alb2/controller/modules"
	ct "alauda.io/alb2/controller/types"
	av1 "alauda.io/alb2/pkg/apis/alauda/v1"
	. "alauda.io/alb2/pkg/controller/ext/otel/types"
	ngt "alauda.io/alb2/pkg/controller/ngxconf/types"
	. "alauda.io/alb2/pkg/utils"
	"alauda.io/alb2/utils"
	jp "github.com/evanphx/json-patch"
	"github.com/go-logr/logr"
	"github.com/xorcare/pointer"
	nv1 "k8s.io/api/networking/v1"
)

const (
	OpenTelemetryEnable            = "nginx.ingress.kubernetes.io/enable-opentelemetry"
	OpenTelemetryTrustIncomingSpan = "nginx.ingress.kubernetes.io/opentelemetry-trust-incoming-spans"
)

type OtelCtl struct {
	Log    logr.Logger
	domain string
}

func NewOtel(log logr.Logger, domain string) *OtelCtl {
	return &OtelCtl{Log: log, domain: domain}
}

// TODO ingress default rule and ft default backend not support otel..

// 根据ingress 生成rule
func (o *OtelCtl) IngressAnnotationToRule(ingress *nv1.Ingress, ruleIndex int, pathIndex int, rule *av1.Rule) {
	domain := o.domain
	if ingress == nil || rule == nil || ingress.Annotations == nil {
		return
	}

	n := config.NewNames(domain)
	val := ingress.Annotations[n.GetAlbIngressOtelAnnotation()]
	if val != "" {
		otel := OtelCrConf{}
		err := json.Unmarshal([]byte(val), &otel)
		if err != nil {
			return
		}
		if !otel.Enable {
			return
		}
		rule.Spec.Config.Otel = &otel
		return
	}

	enable, trust := getIngressOpt(ingress)
	if (enable != nil || trust != nil) && rule.Spec.Config.Otel == nil {
		rule.Spec.Config.Otel = &OtelCrConf{}
	}
	if enable != nil {
		rule.Spec.Config.Otel.Enable = *enable
	}

	if trust != nil {
		if rule.Spec.Config.Otel.Flags == nil {
			rule.Spec.Config.Otel.Flags = &Flags{}
		}
		rule.Spec.Config.Otel.Enable = true
		rule.Spec.Config.Otel.Flags.NoTrustIncomingSpan = !*trust
	}
}

func (o *OtelCtl) ToInternalRule(rule *m.Rule, r *ct.InternalRule) {
	alb_otel, ft_otel, rule_otel := access_otel(rule)
	if !alb_otel.Need() && !rule_otel.Need() && !ft_otel.Need() {
		return
	}
	cf, err := MergeWithDefaultJsonPatch([]*OtelCrConf{alb_otel, ft_otel, rule_otel}, *DEFAULT_OTEL.DeepCopy())
	if err != nil {
		r.Config.Otel = nil
		o.Log.Error(err, "merge otel cfg fail", "rulename", rule.Name)
		return
	}
	if !cf.Enable {
		r.Config.Otel = nil
		return
	}
	r.Config.Otel = &cf.OtelConf
}

func (w *OtelCtl) ToPolicy(r *ct.InternalRule, p *ct.Policy, refs ct.RefMap) {
	if r.Config.Otel == nil {
		return
	}
	p.Config.Otel = r.Config.Otel
}

func (o *OtelCtl) ResolveDnsIfNeed(cf *OtelConf) (*OtelConf, error) {
	if cf.Exporter == nil || cf.Exporter.Collector == nil {
		return nil, fmt.Errorf("invalid otel config, exporter is nil")
	}
	addr, err := ResolveDnsIfNeed(cf.Exporter.Collector.Address)
	if err != nil {
		return nil, err
	}
	cf.Exporter.Collector.Address = addr
	return cf, nil
}

var DEFAULT_OTEL = OtelCrConf{
	OtelConf: OtelConf{
		Exporter: &Exporter{
			BatchSpanProcessor: &BatchSpanProcessor{
				MaxQueueSize:    2048,
				InactiveTimeout: 2,
			},
		},
		Sampler: &Sampler{
			Name: "always_on",
		},
		Flags: &Flags{
			NoTrustIncomingSpan:      false,
			HideUpstreamAttrs:        false,
			ReportHttpRequestHeader:  false,
			ReportHttpResponseHeader: false,
		},
	},
}

func MergeWithDefaultJsonPatch(cfs []*OtelCrConf, default_v OtelCrConf) (OtelCrConf, error) {
	// default < alb < ft < rule
	origin, err := json.Marshal(default_v)
	if err != nil {
		return OtelCrConf{}, err
	}
	for _, c := range cfs {
		if c == nil {
			continue
		}
		p, err := json.Marshal(c)
		if err != nil {
			return OtelCrConf{}, err
		}
		origin, err = jp.MergePatch(origin, p)
		if err != nil {
			return OtelCrConf{}, fmt.Errorf("merge patch fail %v patch %v", err, c)
		}
	}
	cf := OtelCrConf{}
	err = json.Unmarshal(origin, &cf)
	return cf, err
}

// code gen those access method... we donot want to use reflect
func access_otel(r *m.Rule) (alb_otel *OtelCrConf, ft_otel *OtelCrConf, rule_otel *OtelCrConf) {
	ft := r.FT
	alb := ft.LB.Alb
	if alb.Spec.Config != nil && alb.Spec.Config.Otel != nil {
		alb_otel = alb.Spec.Config.Otel
	}
	if ft.Spec.Config != nil && ft.Spec.Config.Otel != nil {
		ft_otel = ft.Spec.Config.Otel
	}
	return alb_otel, ft_otel, r.GetOtel()
}

func ResolveDnsIfNeedWithNet(rawurl string, lookup func(string) ([]string, error)) (string, error) {
	url, err := url.Parse(rawurl)
	if err != nil {
		return "", err
	}
	host := url.Hostname()
	if net.ParseIP(host) != nil {
		return rawurl, nil
	}
	ips, err := lookup(host)
	if err != nil {
		return "", err
	}
	if len(ips) == 0 {
		return "", fmt.Errorf("resolve dns %s fail, no ip", host)
	}
	ip := ips[0]
	for _, p := range ips {
		// prefer ipv4
		if !strings.Contains(p, ":") {
			ip = p
		}
	}
	if utils.IsValidIPv6(ip) {
		ip = "[" + ip + "]"
	}
	if url.Port() == "" {
		url.Host = ip
	} else {
		url.Host = ip + ":" + url.Port()
	}

	return url.String(), nil
}

func ResolveDnsIfNeed(rawurl string) (string, error) {
	return ResolveDnsIfNeedWithNet(rawurl, net.LookupHost)
}

func getIngressOpt(in *nv1.Ingress) (enable, trustincoming *bool) {
	enableAnnot := in.Annotations[OpenTelemetryEnable]
	trustAnnot := in.Annotations[OpenTelemetryTrustIncomingSpan]
	if enableAnnot != "" {
		enable = pointer.Bool(ToBoolOr(enableAnnot, false))
	}
	if trustAnnot != "" {
		trustincoming = pointer.Bool(ToBoolOr(trustAnnot, true))
	}
	return enable, trustincoming
}

func (c *OtelCtl) CollectRefs(ir *ct.InternalRule, refs ct.RefMap) {
}

func (c *OtelCtl) UpdateNgxTmpl(_ *ngt.NginxTemplateConfig, _ *ct.LoadBalancer, _ *config.Config) {
}

func (c *OtelCtl) UpdatePolicyAfterUniq(ext *ct.PolicyExt) {
	if ext.Otel == nil {
		return
	}
	otel, err := c.ResolveDnsIfNeed(ext.Otel)
	if err != nil {
		c.Log.Error(err, "resolve otel dns fail", "policy", ext.Otel)
		ext.Otel = nil
		return
	}
	ext.Otel = otel
}
