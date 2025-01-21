-- format:on style:emmy

local cache = require("config.cache")
local eh = require("error")
local forward_auth = require("plugins.auth.forward_auth")
local basic_auth = require("plugins.auth.basic_auth")

local _m = {
}

---@class AuthCtx
---@field auth_cookie? table the cookie from auth response
---@field always_set_cookie boolean

---@param ctx AlbCtx
function _m.after_rule_match_hook(ctx)
    local auth_cfg, err = _m.get_config(ctx)
    if err ~= nil then
        eh.exit("get auth config fail", err)
        return
    end
    if auth_cfg == nil then
        return
    end
    forward_auth.do_forward_auth_if_need(auth_cfg, ctx)
    basic_auth.do_basic_auth_if_need(auth_cfg, ctx)
end

---@param ctx AlbCtx
function _m.response_header_filter_hook(ctx)
    if ctx.auth == nil then
        return
    end
    forward_auth.add_cookie_if_need(ctx)
end

---@param ctx AlbCtx
---@return AuthPolicy?
---@return any? error
function _m.get_config(ctx)
    return cache.get_config_from_policy(ctx.matched_policy, "auth")
end

return _m
