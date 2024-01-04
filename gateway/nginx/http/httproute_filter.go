package http

import (
	"strings"

	. "alauda.io/alb2/controller/types"
	"github.com/xorcare/pointer"
	gv1a2t "sigs.k8s.io/gateway-api/apis/v1alpha2"
	gv1b1t "sigs.k8s.io/gateway-api/apis/v1beta1"
)

func (h *HttpProtocolTranslate) applyHttpFilterOnRule(ctx HttpCtx, rule *Rule, filters []gv1a2t.HTTPRouteFilter) error {
	log := h.log.WithValues("ctx", ctx.ToString())

	headerModifyFilter := []gv1b1t.HTTPHeaderFilter{}
	redirectFilter := []gv1a2t.HTTPRequestRedirectFilter{}
	rewriteFilter := []gv1a2t.HTTPURLRewriteFilter{}
	// groupby
	for _, f := range filters {
		if f.Type == gv1b1t.HTTPRouteFilterRequestHeaderModifier && f.RequestHeaderModifier != nil {
			headerModifyFilter = append(headerModifyFilter, *f.RequestHeaderModifier)
		}
		if f.Type == gv1b1t.HTTPRouteFilterRequestRedirect && f.RequestRedirect != nil {
			redirectFilter = append(redirectFilter, *f.RequestRedirect)
		}
		if f.Type == gv1b1t.HTTPRouteFilterURLRewrite && f.URLRewrite != nil {
			rewriteFilter = append(rewriteFilter, *f.URLRewrite)
		}
	}
	err := h.applyHeaderModifyFilter(rule, headerModifyFilter)
	if err != nil {
		log.Error(err, "apply header modify filter fail")
	}
	if len(redirectFilter) > 1 {
		log.Info("should only have one http redirect filter")
	}
	if len(redirectFilter) == 1 {
		redirect := redirectFilter[0]
		err = h.applyRedirectFilter(ctx, rule, redirect)
		if err != nil {
			log.Error(err, "apply redirect filter fail")
		}
	}
	if len(rewriteFilter) > 1 {
		log.Info("should only have one http redirect filter")
	}
	if len(rewriteFilter) == 1 {
		rewrite := rewriteFilter[0]
		err = h.applyRewriteFilter(ctx, rule, rewrite)
		if err != nil {
			log.Error(err, "apply rewrite filter fail")
		}
	}
	return nil
}

func (h *HttpProtocolTranslate) applyHeaderModifyFilter(rule *Rule, filters []gv1a2t.HTTPHeaderFilter) error {
	if len(filters) == 0 {
		return nil
	}
	set := map[string]string{}
	add := map[string][]string{}
	remove := []string{}
	for _, f := range filters {
		for _, h := range f.Set {
			set[string(h.Name)] = h.Value
		}
		for _, h := range f.Add {
			name := string(h.Name)
			add[name] = append(add[name], h.Value)
		}
		remove = append(remove, f.Remove...)
	}

	if rule.Config == nil {
		rule.Config = &RuleConfig{}
	}
	rule.Config.RewriteRequest = &RewriteRequestConfig{
		Headers:       set,
		HeadersAdd:    add,
		HeadersRemove: remove,
	}
	return nil
}

func (h *HttpProtocolTranslate) applyRedirectFilter(ctx HttpCtx, rule *Rule, redirect gv1a2t.HTTPRequestRedirectFilter) error {
	if redirect.StatusCode != nil {
		rule.RedirectCode = *redirect.StatusCode
	}
	if redirect.Scheme != nil && *redirect.Scheme != "" {
		rule.RedirectScheme = redirect.Scheme
	}
	if redirect.Hostname != nil && *redirect.Hostname != "" {
		host := string(*redirect.Hostname)
		rule.RedirectHost = &host
	}
	if redirect.Path != nil && redirect.Path.ReplaceFullPath != nil && strings.TrimSpace(*redirect.Path.ReplaceFullPath) != "" {
		fullpath := *redirect.Path.ReplaceFullPath
		rule.RedirectURL = fullpath
	}

	if redirect.Path != nil && redirect.Path.ReplacePrefixMatch != nil && strings.TrimSpace(*redirect.Path.ReplacePrefixMatch) != "" {
		prefixpath := *redirect.Path.ReplacePrefixMatch
		rule.RedirectReplacePrefix = pointer.String(prefixpath)
		match := ctx.GetMatcher()
		path := match.Path
		if path != nil && path.Type != nil && *path.Type == gv1b1t.PathMatchPathPrefix {
			rule.RedirectPrefixMatch = match.Path.Value
		}
	}

	if redirect.Port != nil && *redirect.Port > 0 {
		port := int(*redirect.Port)
		rule.RedirectPort = &port
	}
	return nil
}

func (h *HttpProtocolTranslate) applyRewriteFilter(ctx HttpCtx, rule *Rule, rewrite gv1a2t.HTTPURLRewriteFilter) error {
	if rewrite.Hostname != nil {
		rule.VHost = string(*rewrite.Hostname)
	}
	path := rewrite.Path
	if path == nil {
		return nil
	}
	if path.ReplaceFullPath != nil {
		rule.RewriteBase = ".*"
		rule.RewriteTarget = *rewrite.Path.ReplaceFullPath
		return nil
	}

	if path.ReplacePrefixMatch != nil && *path.ReplacePrefixMatch != "" {
		rule.RewriteReplacePrefix = path.ReplacePrefixMatch
		match := ctx.GetMatcher()
		mpath := match.Path
		if mpath != nil && mpath.Type != nil && *mpath.Type == gv1b1t.PathMatchPathPrefix {
			rule.RewritePrefixMatch = match.Path.Value
		}
	}
	return nil
}
