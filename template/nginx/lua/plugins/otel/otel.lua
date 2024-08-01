-- format:on
-- local _id_generator = require("opentelemetry.trace.id_generator")
local span_kind = require("opentelemetry.trace.span_kind")
local span_status = require("opentelemetry.trace.span_status")
local attr = require("opentelemetry.attribute")
local context = require("opentelemetry.context").new()
local trace_context_propagator = require("opentelemetry.trace.propagation.text_map.trace_context_propagator").new()

local actx = require("ctx.alb_ctx")
local cache = require("config.cache")
local c = require("utils.common")
local lrucache = require "resty.lrucache"
local ot = require("plugins.otel.tracer")
local attribute_set_string = require("plugins.otel.tool").attribute_set_string
local _ = c

---@class OtelCtx
---@field context_token string | nil
---@field cfg OtelConf | nil

local _M = {name = "otel"}

local lru, err = lrucache.new(200) -- 200 different tracers should be enough..
if not lru then
    error("failed to create the cache: " .. (err or "unknown"))
end
---@cast lru -?

local MY_POD_NAME = os.getenv("MY_POD_NAME") or ""
local ALB_NAME = os.getenv("NAME") or ""
local ALB_NS = os.getenv("ALB_NS") or ""
local ALB_VER = os.getenv("ALB_VER") or ""
local HOSTNAME = os.getenv("HOSTNAME") or ""

local l = ngx.log
local E = ngx.ERR

---@param ctx AlbCtx
function _M.is_need(ctx)
    return _M.get_otel_ref(ctx) ~= nil
end

---@param ctx AlbCtx
---@return string | nil
function _M.get_otel_ref(ctx)
    local config = ctx.matched_policy.config
    if config == nil or config.otel == nil or config.otel.otel_ref == nil then
        return nil
    end
    return config.otel.otel_ref
end

---@param ctx AlbCtx
---@return Tracer | nil
---@return string | nil error
function _M.get_tracer_lru(ctx)
    local ref = _M.get_otel_ref(ctx)
    if ref == nil then
        return nil
    end

    local conf, err = cache.get_config(ref)
    if err ~= nil or conf.otel == nil or conf.otel.otel == nil then
        return nil, "[otel] get config error " .. tostring(err)
    end
    local otel = conf.otel.otel
    ctx.otel.cfg = otel

    local hash = ref
    local tracer = lru:get(hash)
    if tracer ~= nil then
        -- l(E, "[otel] get tracer from cache ")
        return tracer
    end

    local resource_attrs = {attr.string("hostname", HOSTNAME), attr.string("service.name", ALB_NAME), attr.string("service.namespace", ALB_NS), attr.string("service.type", "alb"), attr.string("service.version", ALB_VER), attr.string("service.instance.id", MY_POD_NAME)}
    local user_attrs = otel.resource or {}
    -- l(E, "[otel] user attr ", c.json_encode(user_attrs), "\n")
    for k, v in pairs(user_attrs) do
        -- l(E, "[otel] tracer ", "kv ", tostring(k), tostring(v), "\n")
        if v ~= "" then
            table.insert(resource_attrs, attr.string(k, v))
        end
    end
    -- l(E, "[otel] create tracer ", "hash ", hash, " conf ", c.json_encode(otel), "attr", c.json_encode(resource_attrs), "\n")
    local tracer, err = ot.create_tracer(otel, resource_attrs)
    if err ~= nil then
        return nil, "[otel] create tracer fail " .. tostring(err)
    end
    lru:set(hash, tracer, 60 * 60)
    return tracer
end

---@param ctx AlbCtx
---@return OtelCtx
function _M.init_our_ctx(ctx)
    ctx.otel = {context_token = nil, cfg = nil}
    return ctx.otel
end

---@param ctx AlbCtx
---@return OtelCtx | nil
function _M.get_our_ctx(ctx)
    if ctx.otel == nil or ctx.otel.context_token == nil or ctx.otel.cfg == nil then
        return nil
    end
    return ctx.otel
end

---@param ctx AlbCtx
function _M.after_rule_match_hook(ctx)
    if not _M.is_need(ctx) then
        return
    end
    local our = _M.init_our_ctx(ctx)
    local tracer, err = _M.get_tracer_lru(ctx)
    if tracer == nil then
        l(E, "[otel] get tracer fail ", tostring(err))
        ctx.otel = nil
        return
    end
    local upstream_context = context
    local trust = not our.cfg.flags.notrust_incoming_span
    if trust then
        upstream_context = trace_context_propagator:extract(context, ngx.req)
    end
    -- TODO add more attributes via config
    local attributes = {attr.string("net.host.name", ctx.var.host), attr.string("http.request.method", ctx.var.method), attr.string("http.scheme", ctx.var.scheme), attr.string("http.target", ctx.var.request_uri), attr.string("http.user_agent", ctx.var.http_user_agent)}

    _M.inject_rule_source_attribute(ctx, attributes)

    local span_name = ctx.var.method .. " " .. ctx.var.request_uri
    local otel_ctx = tracer:start(upstream_context, span_name, {kind = span_kind.server, attributes = attributes})
    if otel_ctx == nil then
        l(E, "[otel] start span failed")
        return
    end

    our.context_token = otel_ctx:attach()
    trace_context_propagator:inject(otel_ctx, ngx.req)
end

---@param ctx AlbCtx
function _M.log_hook(ctx)
    local otel = _M.get_our_ctx(ctx)
    if otel == nil then
        return
    end

    local otel_ctx = context:current()
    local span = otel_ctx:span()
    local upstream_status = actx.get_last_upstream_status(ctx)
    if upstream_status and upstream_status >= 500 then
        span:set_status(span_status.ERROR, "upstream response status: " .. upstream_status)
    end

    span:set_attributes(attr.int("http.status_code", upstream_status))
    span:set_attributes(attr.int("http.request.resend_count", ctx.send_count - 1))
    _M.inject_upstream_attribute(otel.cfg.flags, ctx.peer, span)
    _M.inject_http_header(otel.cfg.flags, span)
    span:finish()
end

---@param ctx AlbCtx
---@param attrs table
function _M.inject_rule_source_attribute(ctx, attrs)
    attribute_set_string(attrs, "alb.rule.rule_name", ctx.matched_policy.rule)
    attribute_set_string(attrs, "alb.rule.source_type", ctx.matched_policy.source_type)
    attribute_set_string(attrs, "alb.rule.source_name", ctx.matched_policy.source_name)
    attribute_set_string(attrs, "alb.rule.source_ns", ctx.matched_policy.source_ns)
end

---@param flags Flags
---@param peer PeerConf
---@param span any
function _M.inject_upstream_attribute(flags, peer, span)
    if flags.hide_upstream_attrs then
        return
    end
    local attrs = {}
    attribute_set_string(attrs, "alb.upstream.svc_name", peer.conf.svc)
    attribute_set_string(attrs, "alb.upstream.svc_ns", peer.conf.ns)
    attribute_set_string(attrs, "alb.upstream.peer", peer.peer)
    for _, a in ipairs(attrs) do
        span:set_attributes(a)
    end
end

---@param flags Flags
---@param span any
function _M.inject_http_header(flags, span)
    local attrs = {}
    if flags.report_http_reqeust_header then
        for k, v in pairs(ngx.req.get_headers()) do
            attribute_set_string(attrs, "http.request.header." .. k, tostring(v))
        end
    end
    if flags.report_http_response_header then
        for k, v in pairs(ngx.resp.get_headers()) do
            attribute_set_string(attrs, "http.response.header." .. k, tostring(v))
        end
    end

    for _, a in ipairs(attrs) do
        span:set_attributes(a)
    end
end
return _M
