package http

import (
	. "alauda.io/alb2/controller/types"
	gatewayType "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

func (h *HttpProtocolTranslate) applyHttpFilterOnRule(ctx HttpCtx, rule *Rule, filters []gatewayType.HTTPRouteFilter) error {
	log := h.log.WithValues("ctx", ctx.ToString())

	headerModifyFilter := []gatewayType.HTTPRequestHeaderFilter{}
	redirectFilter := []gatewayType.HTTPRequestRedirectFilter{}
	// groupby
	for _, f := range filters {
		if f.Type == gatewayType.HTTPRouteFilterRequestHeaderModifier && f.RequestHeaderModifier != nil {
			headerModifyFilter = append(headerModifyFilter, *f.RequestHeaderModifier)
		}
		if f.Type == gatewayType.HTTPRouteFilterRequestRedirect && f.RequestRedirect != nil {
			redirectFilter = append(redirectFilter, *f.RequestRedirect)
		}
	}
	err := h.applyHeaderModifyFilter(rule, headerModifyFilter)
	if err != nil {
		log.Error(err, "apply header modify filter fail")
	}
	if len(redirectFilter) == 0 {
		return nil
	}
	if len(redirectFilter) > 1 {
		log.Info("should only have one http redirect filter")
	}
	redirect := redirectFilter[0]
	err = h.applyRedirectFilter(rule, redirect)
	if err != nil {
		log.Error(err, "apply redirect filter fail")
	}
	return nil
}

func (h *HttpProtocolTranslate) applyHeaderModifyFilter(rule *Rule, filters []gatewayType.HTTPRequestHeaderFilter) error {
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
	rule.Config.RewriteResponse = &RewriteResponseConfig{
		Headers:       set,
		HeadersAdd:    add,
		HeadersRemove: remove,
	}
	return nil
}

func (h *HttpProtocolTranslate) applyRedirectFilter(rule *Rule, redirect gatewayType.HTTPRequestRedirectFilter) error {
	// TODO: webhook
	if redirect.StatusCode != nil {
		rule.RedirectCode = int(*redirect.StatusCode)
	}
	if redirect.Scheme != nil && *redirect.Scheme != "" {
		rule.RedirectScheme = redirect.Scheme
	}
	if redirect.Hostname != nil && *redirect.Hostname != "" {
		host := string(*redirect.Hostname)
		rule.RedirectHost = &host
	}
	if redirect.Port != nil && *redirect.Port > 0 {
		port := int(*redirect.Port)
		rule.RedirectPort = &port
	}
	return nil
}
