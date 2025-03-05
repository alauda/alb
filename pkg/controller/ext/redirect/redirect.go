package redirect

import (
	"fmt"
	"strconv"

	"github.com/go-logr/logr"
	"github.com/xorcare/pointer"

	m "alauda.io/alb2/controller/modules"
	"alauda.io/alb2/ingress/util"

	ct "alauda.io/alb2/controller/types"

	albv1 "alauda.io/alb2/pkg/apis/alauda/v1"

	. "alauda.io/alb2/pkg/controller/ext/redirect/types"
	lu "alauda.io/alb2/utils"
	nv1 "k8s.io/api/networking/v1"

	et "alauda.io/alb2/pkg/controller/extctl/types"
)

// redirect 特殊之处在于
// 当在ingress上配置了ssl-redirect,在ingress 同步时，如果ft是https的，并且只配置ssl-redirect，那么实际上不应该做redirect.
// 当在ft上配置了ssl-redirect,如果没有默认后端路由,要创建一个做redirect的policy,并且这个policy要优先匹配.
// 我们在alb单独提供一个ingress-ssl-redirect的开关，来让ingress-http的ft做ssl-redirect.
// ssl-redirect 对我们来说实际上是redirect.scheme=https.
type RedirectCtl struct {
	log    logr.Logger
	domain string
}

func NewRedirectCtl(log logr.Logger, domain string) et.ExtensionInterface {
	x := &RedirectCtl{
		log:    log,
		domain: domain,
	}
	return et.ExtensionInterface{
		InitL7Ft:                      x.InitL7Ft,
		NeedL7DefaultPolicy:           x.NeedL7DefaultPolicy,
		InitL7DefaultPolicy:           x.InitL7DefaultPolicy,
		IngressWithFtAnnotationToRule: x.IngressWithFtAnnotationToRule,
		ToInternalRule:                x.ToInternalRule,
		ToPolicy:                      x.ToPolicy,
	}
}

// 用户可以在ft上指定某个ft做ssl-redirect
// 也可以在alb上指定做ingress的那个http的ft做ssl-redirect # 这种场景比较适合在部署的时候指定
// 初始化ft的redirect配置。处理alb/ft的redirect的merge逻辑
func (t *RedirectCtl) InitL7Ft(mft *m.Frontend, cft *ct.Frontend) {
	if redirect := getRedirectFromFt(mft); redirect != nil {
		cft.Config.Redirect = redirect
		return
	}
	if isIngressSSLRedirectFrontend(mft) {
		cft.Config.Redirect = &RedirectCr{
			Code:   pointer.Int(308),
			Scheme: "https",
		}
		return
	}
}

// 如果ft上有redirect的配置,即使没有默认后端路由我们也要创建一个默认的转发规则
func (t *RedirectCtl) NeedL7DefaultPolicy(cft *ct.Frontend) (bool, string) {
	if cft.Config.Redirect != nil {
		return true, "redirect"
	}
	return false, ""
}

// redirect不存在继承性
// 如果在ft上配置了redirect。实际上会创建一个优先级最高的policy，直接将所有向这个端口请求的流量做redirect
func (t *RedirectCtl) InitL7DefaultPolicy(cft *ct.Frontend, policy *ct.Policy) {
	if cft.Config.Redirect == nil {
		return
	}
	policy.MakeItMatchFirst()
	policy.Config.Redirect = cft.Config.Redirect
}

func (t *RedirectCtl) IngressWithFtAnnotationToRule(ft *albv1.Frontend, ing *nv1.Ingress, rindex int, pindex int, rule *albv1.Rule) {
	redirect, err := t.parseIngressRedirect(ing, rindex, pindex)
	if err != nil {
		t.log.Error(err, "ingress redirect annotation parse failed")
		return
	}

	if redirect == nil {
		return
	}

	redirectCR := t.buildRedirectCR(redirect, ing, rindex)
	if redirectCR == nil {
		return
	}
	// Skip SSL redirect for HTTPS frontend
	t.log.V(3).Info("redirectCR", "redirectCR", lu.PrettyJson(redirectCR), "onlySslRedirect", redirectCR.OnlySslRedirect(int(ft.Spec.Port)), "port", ft.Spec.Port, "protocol", ft.Spec.Protocol)
	if ft.Spec.Protocol == "https" && redirectCR.OnlySslRedirect(int(ft.Spec.Port)) {
		return
	}

	rule.Spec.Config.Redirect = redirectCR
}

func (t *RedirectCtl) parseIngressRedirect(ing *nv1.Ingress, rindex int, pindex int) (*RedirectIngress, error) {
	var redirect RedirectIngress
	prefix := []string{fmt.Sprintf("index.%d-%d.alb.ingress.%s", rindex, pindex, t.domain), fmt.Sprintf("alb.ingress.%s", t.domain), "nginx.ingress.kubernetes.io"}
	has, err := ResolverRedirectIngressFromAnnotation(&redirect, ing.Annotations, prefix)
	if err != nil || !has {
		return nil, err
	}
	return &redirect, nil
}

func (t *RedirectCtl) buildRedirectCR(redirect *RedirectIngress, ingress *nv1.Ingress, rindex int) *RedirectCr {
	cr := &RedirectCr{}

	err := ReAssignRedirectIngressToRedirectCr(redirect, cr, &ReAssignRedirectIngressToRedirectCrOpt{
		String_to_int: func(port string) (*int, error) {
			if port == "" {
				return nil, nil
			}
			return stringToIntOr(port, 308), nil
		},
	})
	if err != nil {
		t.log.Error(err, "buildRedirectCR failed")
		return nil
	}

	// Handle permanent redirect
	if redirect.PermanentRedirect != "" {
		cr.URL = redirect.PermanentRedirect
		cr.Code = stringToIntOr(redirect.PermanentRedirectCode, 301)
	}

	// Handle temporal redirect
	if redirect.TemporalRedirect != "" {
		cr.URL = redirect.TemporalRedirect
		cr.Code = stringToIntOr(redirect.TemporalRedirectCode, 302)
	}

	// Handle SSL redirect
	if redirect.ForceSSLRedirect == "true" {
		cr.Scheme = "https"
		if cr.Code == nil {
			cr.Code = pointer.Int(308)
		}
	}
	// https://github.com/kubernetes/ingress-nginx/blob/main/docs/user-guide/nginx-configuration/annotations.md#server-side-https-enforcement-through-redirect
	if redirect.SSLRedirect == "true" && t.hasTls(ingress, rindex) {
		cr.Scheme = "https"
		if cr.Code == nil {
			cr.Code = pointer.Int(308)
		}
	}
	// redirect即使ingress上有某些annotation,但是不一定代表我们一定要做redirect.
	// 比如ingress上配置了ssl-redirect=true,但是没有配置证书,那么实际上不应该做redirect.
	if cr.Empty() {
		return nil
	}
	return cr
}

func (t *RedirectCtl) hasTls(ingress *nv1.Ingress, rindex int) bool {
	if ingress.Spec.Rules == nil || len(ingress.Spec.Rules) <= rindex {
		return false
	}
	host := ingress.Spec.Rules[rindex].Host
	if host == "" {
		return false
	}
	return t.hasTlsFromSpec(ingress, host) || t.hasTlsFromAnnotation(ingress, host)
}

func (t *RedirectCtl) hasTlsFromSpec(ingress *nv1.Ingress, host string) bool {
	if ingress.Spec.TLS == nil {
		return false
	}
	for _, tls := range ingress.Spec.TLS {
		for _, h := range tls.Hosts {
			if h == host {
				return true
			}
		}
	}
	return false
}

func (t *RedirectCtl) hasTlsFromAnnotation(ingress *nv1.Ingress, host string) bool {
	if ingress.Annotations == nil {
		return false
	}
	ALBSSLAnnotation := fmt.Sprintf("alb.networking.%s/tls", t.domain)
	ssl := ingress.Annotations[ALBSSLAnnotation]
	if ssl == "" {
		return false
	}
	sslMap := util.ParseSSLAnnotation(ssl)
	if sslMap == nil {
		return false
	}
	if _, ok := sslMap[host]; ok {
		return true
	}
	return false
}

func (t *RedirectCtl) ToInternalRule(rule *m.Rule, ir *ct.InternalRule) {
	if !hasRedirectConfig(rule) {
		return
	}
	// 将redirect的配置整理到.config.redirect下.同时兼容旧的配置
	redirect := mergeLegacyRedirectConfig(rule)
	ir.Config.Redirect = &redirect
}

func (t *RedirectCtl) ToPolicy(ir *ct.InternalRule, p *ct.Policy, refs ct.RefMap) {
	if ir.Config.Redirect == nil {
		return
	}
	p.Config.Redirect = ir.Config.Redirect
}

// tools

func hasRedirectConfig(rule *m.Rule) bool {
	return rule.Spec.RedirectCode != 0 || rule.Spec.RedirectURL != "" || rule.Spec.Config.Redirect != nil
}

func mergeLegacyRedirectConfig(rule *m.Rule) RedirectCr {
	redirect := RedirectCr{}
	if rule.Spec.Config.Redirect != nil {
		redirect = *rule.Spec.Config.Redirect
	}
	if rule.Spec.RedirectCode != 0 {
		redirect.Code = pointer.Int(rule.Spec.RedirectCode)
	}
	if rule.Spec.RedirectURL != "" {
		redirect.URL = rule.Spec.RedirectURL
	}
	return redirect
}

func getRedirectFromFt(mft *m.Frontend) *RedirectCr {
	if mft.GetFtConfig() == nil {
		return nil
	}
	return mft.GetFtConfig().Redirect
}

func isIngressSSLRedirectFrontend(mft *m.Frontend) bool {
	if mft.Spec.Protocol != "http" {
		return false
	}

	albConfig := mft.GetAlbConfig()
	if albConfig == nil {
		return false
	}
	if albConfig.IngressSSLRedirect == nil {
		return false
	}
	if !*albConfig.IngressSSLRedirect {
		return false
	}

	// Check if ingress HTTP is disabled
	if albConfig.IngressHTTPPort != nil && *albConfig.IngressHTTPPort == 0 {
		return false
	}

	// Check if this is the configured ingress HTTP port
	if albConfig.IngressHTTPPort != nil {
		return *albConfig.IngressHTTPPort == int(mft.Spec.Port)
	}

	// Default case: port 80 is the ingress HTTP port
	return mft.Spec.Port == 80
}

func stringToIntOr(s string, default_code int) *int {
	if s == "" {
		return pointer.Int(default_code)
	}
	code, err := strconv.Atoi(s)
	if err != nil {
		return pointer.Int(default_code)
	}
	return pointer.Int(code)
}
