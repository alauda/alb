package http

import (
	"strings"

	"alauda.io/alb2/controller/types"
	"github.com/xorcare/pointer"
	gv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func (h *HttpProtocolTranslate) applyHttpFilterOnRule(ctx HttpCtx, rule *types.InternalRule, filters []gv1.HTTPRouteFilter) error {
	log := h.log.WithValues("ctx", ctx.ToString())

	headerModifyFilter := []gv1.HTTPHeaderFilter{}
	redirectFilter := []gv1.HTTPRequestRedirectFilter{}
	rewriteFilter := []gv1.HTTPURLRewriteFilter{}
	// groupby
	for _, f := range filters {
		if f.Type == gv1.HTTPRouteFilterRequestHeaderModifier && f.RequestHeaderModifier != nil {
			headerModifyFilter = append(headerModifyFilter, *f.RequestHeaderModifier)
		}
		if f.Type == gv1.HTTPRouteFilterRequestRedirect && f.RequestRedirect != nil {
			redirectFilter = append(redirectFilter, *f.RequestRedirect)
		}
		if f.Type == gv1.HTTPRouteFilterURLRewrite && f.URLRewrite != nil {
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

func (h *HttpProtocolTranslate) applyHeaderModifyFilter(rule *types.InternalRule, filters []gv1.HTTPHeaderFilter) error {
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

	rule.Config.RewriteRequest = &types.RewriteRequestConfig{
		Headers:       set,
		HeadersAdd:    add,
		HeadersRemove: remove,
	}
	return nil
}

func (h *HttpProtocolTranslate) applyRedirectFilter(ctx HttpCtx, r *types.InternalRule, redirect gv1.HTTPRequestRedirectFilter) error {
	if r.Config.Redirect == nil {
		r.Config.Redirect = &types.RedirectConf{}
	}
	rule := r.Config.Redirect
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
		fullPath := *redirect.Path.ReplaceFullPath
		rule.RedirectURL = fullPath
	}

	if redirect.Path != nil && redirect.Path.ReplacePrefixMatch != nil && strings.TrimSpace(*redirect.Path.ReplacePrefixMatch) != "" {
		prefixPath := *redirect.Path.ReplacePrefixMatch
		rule.RedirectReplacePrefix = pointer.String(prefixPath)
		match := ctx.GetMatcher()
		path := match.Path
		if path != nil && path.Type != nil && *path.Type == gv1.PathMatchPathPrefix {
			rule.RedirectPrefixMatch = match.Path.Value
		}
	}

	if redirect.Port != nil && *redirect.Port > 0 {
		port := int(*redirect.Port)
		rule.RedirectPort = &port
	}
	return nil
}

func (h *HttpProtocolTranslate) applyRewriteFilter(ctx HttpCtx, r *types.InternalRule, rewrite gv1.HTTPURLRewriteFilter) error {
	if r.Config.Vhost == nil {
		r.Config.Vhost = &types.Vhost{}
	}

	if rewrite.Hostname != nil {
		r.Config.Vhost.VHost = string(*rewrite.Hostname)
	}
	path := rewrite.Path
	if path == nil {
		return nil
	}
	if r.Config.Rewrite == nil {
		r.Config.Rewrite = &types.RewriteConf{}
	}
	rule := r.Config.Rewrite
	if path.ReplaceFullPath != nil {
		rule.RewriteBase = ".*"
		rule.RewriteTarget = *rewrite.Path.ReplaceFullPath
		return nil
	}

	if path.ReplacePrefixMatch != nil && *path.ReplacePrefixMatch != "" {
		rule.RewriteReplacePrefix = path.ReplacePrefixMatch
		matchPath := ctx.GetMatcher().Path
		if matchPath != nil && matchPath.Type != nil && *matchPath.Type == gv1.PathMatchPathPrefix {
			rule.RewritePrefixMatch = matchPath.Value
		}
	}
	return nil
}
