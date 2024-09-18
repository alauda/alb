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
	. "alauda.io/alb2/pkg/utils"
	"alauda.io/alb2/utils"
	u "alauda.io/alb2/utils"
	jp "github.com/evanphx/json-patch"
	"github.com/go-logr/logr"
	"github.com/xorcare/pointer"
	nv1 "k8s.io/api/networking/v1"
)

const (
	OpenTelemetryEnable            = "nginx.ingress.kubernetes.io/enable-opentelemetry"
	OpenTelemetryTrustIncomingSpan = "nginx.ingress.kubernetes.io/opentelemetry-trust-incoming-spans"
)

type Otel struct {
	Log logr.Logger
}

func NewOtel(log logr.Logger) *Otel {
	return &Otel{Log: log}
}

// TODO ingress default rule and ft default backend not support otel..

// 根据ingress 生成rule
func (o *Otel) UpdateRuleViaIngress(ingress *nv1.Ingress, ruleIndex int, pathIndex int, rule *av1.Rule, domain string) {
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

// rule cr 转成 policy
func (o *Otel) FromRuleCr(rule *m.Rule, r *ct.Rule) {
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
	if r.Config.Otel == nil {
		r.Config.Otel = &OtelInPolicy{}
	}
	hash := Hash(u.PrettyJson(cf))
	// IMPR 这样的一个问题是 从policy上我们不是很容易看出这个hash属于谁
	r.Config.Otel.Hash = hash
	r.Config.Otel.Otel = &cf.OtelConf
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

// 有所有policy的情况下重新整理
func (o *Otel) ResolvePolicy(alb *ct.LoadBalancer, policy *ct.NgxPolicy) error {
	// 遍历所有的rule 如果配置相同 提取成同一个 config 并设置好对应ref

	type ResolvedOtel struct {
		otel OtelConf
		err  error
	}
	common_otel := map[string]ResolvedOtel{}

	resolve := func(hash string, otel *OtelConf) error {
		if c, has := common_otel[hash]; has {
			return c.err
		}
		addr, err := ResolveDnsIfNeed(otel.Exporter.Collector.Address)
		if err != nil {
			common_otel[hash] = ResolvedOtel{err: err}
			return err
		}
		otel.Exporter.Collector.Address = addr
		common_otel[hash] = ResolvedOtel{
			otel: *otel,
		}
		return nil
	}
	for _, ps := range policy.Http.Tcp {
		for _, p := range ps {
			otel := p.GetOtel()
			if otel == nil {
				continue
			}
			// set otel in matched policy to nil, we should get it via ref
			p.Config.Otel.Otel = nil
			if !otel.HasCollector() {
				continue
			}
			hash := p.Config.Otel.Hash
			if resolve(hash, otel) != nil {
				continue
			}
			p.Config.Otel.OtelRef = pointer.String(hash)
		}
	}

	for hash, otel := range common_otel {
		if otel.err != nil {
			continue
		}
		policy.CommonConfig[hash] = ct.CommonPolicyConfigVal{
			Type: "otel",
			Otel: &OtelInCommon{
				Otel: otel.otel,
			},
		}
	}
	return nil
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
