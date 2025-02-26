-- format:on style:emmy

---@class Plugin
---@field after_rule_match_hook fun(ctx: AlbCtx)
---@field response_header_filter_hook fun(ctx: AlbCtx)
---@field log_hook fun(ctx: AlbCtx)
---@class PluginManager
---@field plugins { [string]: Plugin }
---@field name string
local _m = { plugins = {} } -- self_plugins need to access via : syntax

function _m.init()
    local otel = require("plugins.otel.otel")
    local auth = require("plugins.auth.auth")
    local timeout = require("plugins.timeout")
    _m.plugins = {
        ["auth"] = auth,
        ["otel"] = otel,
        ["timeout"] = timeout,
    }
end

---@param ctx AlbCtx
---@param hook string
function _m._call_hook(ctx, hook)
    local plugins = ctx.matched_policy.plugins or {}
    for _, p_name in ipairs(plugins) do
        local p = _m.plugins[p_name]
        if p ~= nil and p[hook] then
            p[hook](ctx)
        end
    end
end

---@param ctx AlbCtx
function _m.after_rule_match_hook(ctx)
    _m._call_hook(ctx, "after_rule_match_hook")
end

---@param ctx AlbCtx
function _m.response_header_filter_hook(ctx)
    _m._call_hook(ctx, "response_header_filter_hook")
end

---@param ctx AlbCtx
function _m.log_hook(ctx)
    _m._call_hook(ctx, "log_hook")
end

function _m.balancer_hook(ctx)
    _m._call_hook(ctx, "balancer_hook")
end

_m.init()
return _m
