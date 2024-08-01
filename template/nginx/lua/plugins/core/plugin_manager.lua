-- format:on
---@class Plugin
---@field after_rule_match_hook fun(ctx: AlbCtx)
---@field header_filter_hook fun(ctx: AlbCtx)
---@field log_hook fun(ctx: AlbCtx)
---@class PluginManager
---@field plugins { [string]: Plugin }
---@field name string
local _m = {plugins = {}, self_plugins = {}} -- self_plugins need to access via : syntax

function _m.init()
    local otel = require("plugins.otel.otel")
    table.insert(_m.plugins, otel)
end

---@param ctx AlbCtx
function _m.after_rule_match_hook(ctx)
    for _, p in ipairs(_m.plugins) do
        if p.after_rule_match_hook then
            p.after_rule_match_hook(ctx)
        end
    end
end

---@param ctx AlbCtx
function _m.header_filter_hook(ctx)
    for _, p in ipairs(_m.plugins) do
        if p.header_filter_hook then
            p.header_filter_hook(ctx)
        end
    end
end

---@param ctx AlbCtx
function _m.log_hook(ctx)
    for _, p in ipairs(_m.plugins) do
        if p.log_hook then
            p.log_hook(ctx)
        end
    end
end

_m.init()
return _m
