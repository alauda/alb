package waf

import (
	"fmt"
	"sort"
	"strings"

	"alauda.io/alb2/config"
	. "alauda.io/alb2/controller/types"
	. "alauda.io/alb2/pkg/controller/ext/waf/types"
	. "alauda.io/alb2/pkg/controller/ngxconf/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	m "alauda.io/alb2/controller/modules"
	ct "alauda.io/alb2/controller/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	av1 "alauda.io/alb2/pkg/apis/alauda/v1"
	"github.com/go-logr/logr"
	nv1 "k8s.io/api/networking/v1"
)

type WafCtl struct {
	log    logr.Logger
	domain string
}

func NewWaf(l logr.Logger, domain string) *WafCtl {
	return &WafCtl{log: l, domain: domain}
}

const (
	EnableModSecurity        = "nginx.ingress.kubernetes.io/enable-modsecurity"
	EnableOwaspCoreRules     = "nginx.ingress.kubernetes.io/enable-owasp-core-rules"
	ModSecurityTransactionID = "nginx.ingress.kubernetes.io/modsecurity-transaction-id"
	ModSecuritySnippet       = "nginx.ingress.kubernetes.io/modsecurity-snippet"
)

// 根据ingress 生成rule
func (w *WafCtl) IngressAnnotationToRule(ingress *nv1.Ingress, ruleIndex int, pathIndex int, rule *av1.Rule) {
	domain := w.domain
	if ingress == nil || rule == nil || ingress.Annotations == nil {
		return
	}
	n := config.NewNames(domain)
	if ingress.Annotations[EnableModSecurity] == "" {
		return
	}
	enable := ingress.Annotations[EnableModSecurity] == "true"
	waf := WafCrConf{
		Enable: enable,
	}
	wafcf := WafConf{}
	hascf := false
	if ingress.Annotations[EnableOwaspCoreRules] == "true" {
		hascf = true
		wafcf.UseCoreRules = true
	}
	if ingress.Annotations[ModSecurityTransactionID] != "" {
		hascf = true
		wafcf.TransactionId = ingress.Annotations[ModSecurityTransactionID]
	}
	if ingress.Annotations[ModSecuritySnippet] != "" {
		hascf = true
		rule.Annotations[ModSecuritySnippet] = ingress.Annotations[ModSecuritySnippet]
	}
	if ingress.Annotations[n.GetAlbWafCmRefAnnotation()] != "" {
		hascf = true
		wafcf.CmRef = ingress.Annotations[n.GetAlbWafCmRefAnnotation()]
	}
	if ingress.Annotations[n.GetAlbWafUseRecommendAnnotation()] != "" {
		hascf = true
		wafcf.UseRecommend = ingress.Annotations[n.GetAlbWafUseRecommendAnnotation()] == "true"
	}
	if hascf {
		waf.WafConf = wafcf
	}
	if rule.Spec.Config == nil {
		rule.Spec.Config = &av1.RuleConfigInCr{}
	}
	rule.Spec.Config.ModeSecurity = &waf
}

// rule cr 转成 policy
func (w *WafCtl) ToInternalRule(rule *m.Rule, r *ct.InternalRule) {
	waf, snip, key := mergeWaf(rule)
	if waf == nil || !waf.Enable {
		return
	}
	r.Config.Waf = &WafInternal{
		Raw:     waf.WafConf,
		Snippet: snip,
		Key:     key,
	}
	r.Config.Source[ct.Waf] = key
}

func (w *WafCtl) ToPolicy(rule *ct.InternalRule, r *ct.Policy, refs ct.RefMap) {
	if rule.Config.Waf == nil {
		return
	}
	r.ToLocation = &rule.Config.Waf.Key
}

func getWafAnnotation(obj metav1.ObjectMeta) string {
	return obj.Annotations[ModSecuritySnippet]
}

func mergeWaf(r *m.Rule) (*WafCrConf, string, string) {
	if r.GetWaf() != nil {
		waf := r.GetWaf()
		snip := getWafAnnotation(r.Rule.ObjectMeta)
		if r.Spec.Source != nil {
			source := r.Spec.Source
			key := fmt.Sprintf("waf_ing_%s_%s", source.Namespace, source.Name)
			return waf, snip, key
		}
		return waf, snip, fmt.Sprintf("waf_rule_%s", r.Name)
	}
	if r.GetFtConfig() != nil && r.GetFtConfig().ModeSecurity != nil {
		return r.GetFtConfig().ModeSecurity, getWafAnnotation(r.FT.ObjectMeta), fmt.Sprintf("waf_ft_%s", r.FT.Name)
	}
	if r.GetAlbConfig() != nil && r.GetAlbConfig().ModeSecurity != nil {
		return r.GetAlbConfig().ModeSecurity, getWafAnnotation(r.FT.LB.Alb.ObjectMeta), fmt.Sprintf("waf_alb_%s", r.FT.Name)
	}
	return nil, "", ""
}

func (w *WafCtl) CollectRefs(r *ct.InternalRule, refs RefMap) {
	waf := r.Config.Waf
	if waf == nil || waf.Raw.CmRef == "" {
		return
	}
	ns, name, _, err := ParseCmRef(waf.Raw.CmRef)
	if err != nil {
		w.log.Error(err, "invalid cmref", "cmref", waf.Raw.CmRef)
		return
	}
	key := client.ObjectKey{Namespace: ns, Name: name}
	refs.ConfigMap[key] = nil
}

// 更新和删除配置不会有问题，因为现在的custom location的名字是固定的
// 唯一可能的就是刚刚增加的时候，有可能旧的worker看到nginx 配置还是旧的。但是读到的policy已经是新的了。导致想去跳转到一个还不存在的location上去。
// 只对长连接有影响。
func (w *WafCtl) UpdateNgxTmpl(tmpl_cfg *NginxTemplateConfig, alb *LoadBalancer, cfg *config.Config) {
	custom_location := map[string]map[string]FtCustomLocation{}
	for _, f := range alb.Frontends {
		for _, r := range f.Rules {
			waf := r.Config.Waf
			if waf == nil {
				continue
			}
			if _, ok := custom_location[f.String()]; !ok {
				custom_location[f.String()] = map[string]FtCustomLocation{}
			}
			key := waf.Key
			if _, has := custom_location[f.String()][key]; !has {
				custom_location[f.String()][key] = FtCustomLocation{
					Name:        key,
					LocationRaw: GenLocation(alb.Refs, r),
				}
			}
		}
	}
	for f, ftmap := range custom_location {
		if _, has := tmpl_cfg.Frontends[f]; !has {
			w.log.Info("ft not find?", "ft", f)
			continue
		}
		ft := tmpl_cfg.Frontends[f]
		waf_custom := []FtCustomLocation{}
		for _, raw := range ftmap {
			waf_custom = append(waf_custom, raw)
		}
		ft.CustomLocation = append(ft.CustomLocation, waf_custom...)
		// make sure location are ordered
		sort.Slice(ft.CustomLocation, func(i, j int) bool {
			return ft.CustomLocation[i].Name < ft.CustomLocation[j].Name
		})
		tmpl_cfg.Frontends[f] = ft
	}
}

func GenLocation(cms RefMap, r *ct.InternalRule) string {
	waf := r.Config.Waf
	if waf.Snippet != "" {
		waf.Raw.CmRef = ""
		waf.Raw.UseCoreRules = false
	}
	if waf.Raw.CmRef != "" {
		waf.Raw.UseCoreRules = false
	}
	pickCm := func(cms RefMap, ref string) string {
		if ref == "" {
			return ""
		}
		ns, name, section, err := ParseCmRef(ref)
		if err != nil {
			return ""
		}
		key := client.ObjectKey{
			Namespace: ns,
			Name:      name,
		}
		if cm, has := cms.ConfigMap[key]; has {
			return cm.Data[section]
		}
		return ""
	}
	cm := pickCm(cms, waf.Raw.CmRef)
	snip := ""
	if waf.Snippet != "" {
		snip = "modsecurity_rules '" + "\n" + waf.Snippet + "\n" + "';"
	}
	if cm != "" {
		cm = "modsecurity_rules '" + "\n" + cm + "\n" + "';"
	}
	coreruleset := ""
	if waf.Raw.UseCoreRules {
		coreruleset = "modsecurity_rules_file /etc/nginx/owasp-modsecurity-crs/nginx-modsecurity.conf;"
	}
	trans_id := ""
	recommend := ""
	if waf.Raw.UseRecommend {
		recommend = "modsecurity_rules_file /etc/nginx/modsecurity/modsecurity.conf;"
	}
	if waf.Raw.TransactionId != "" {
		trans_id = fmt.Sprintf("modsecurity_transaction_id \"%s\";", waf.Raw.TransactionId)
	}
	return fmt.Sprintf(`
modsecurity on;
%s
%s
%s
%s
%s
	`, recommend, trans_id, cm, snip, coreruleset,
	)
}

func ParseCmRef(ref string) (ns, name, section string, err error) {
	s := strings.Split(ref, "/")
	if len(s) != 2 {
		return "", "", "", fmt.Errorf("invalid cmref %v", s)
	}
	ns = s[0]
	name_and_section := strings.SplitN(s[1], "#", 2)
	if len(name_and_section) != 2 {
		return "", "", "", fmt.Errorf("invalid cmref %v", s)
	}
	name = name_and_section[0]
	section = name_and_section[1]
	return ns, name, section, nil
}

func (c *WafCtl) UpdatePolicyAfterUniq(ext *ct.PolicyExt) {
}
