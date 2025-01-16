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

-- [nginx.ingress.kubernetes.io/auth-keepalive]
-- [nginx.ingress.kubernetes.io/auth-keepalive-requests]
-- [nginx.ingress.kubernetes.io/auth-keepalive-timeout]

-- [nginx.ingress.kubernetes.io/auth-realm]
-- [nginx.ingress.kubernetes.io/auth-secret]
-- [nginx.ingress.kubernetes.io/auth-secret-type]
-- [nginx.ingress.kubernetes.io/auth-type]


-- [nginx.ingress.kubernetes.io/auth-url]
-- [nginx.ingress.kubernetes.io/auth-method]
-- [nginx.ingress.kubernetes.io/auth-proxy-set-headers] # 从configmap中获取，go部分将其转换成具体的map
-- [nginx.ingress.kubernetes.io/auth-request-redirect]
-- [nginx.ingress.kubernetes.io/auth-response-headers]
-- [nginx.ingress.kubernetes.io/auth-signin]
-- [nginx.ingress.kubernetes.io/auth-always-set-cookie]
-- [nginx.ingress.kubernetes.io/auth-signin-redirect-param] # go部分根据这个annotation修改sign的var_string
-- not supported
-- [nginx.ingress.kubernetes.io/auth-snippet]
-- [nginx.ingress.kubernetes.io/auth-cache-duration]
-- [nginx.ingress.kubernetes.io/auth-cache-key]

---
---@param ctx AlbCtx
---@return AuthPolicy?
---@return any? error
function _m.get_config(ctx)
    return cache.get_config_from_policy(ctx.matched_policy, "auth")
end

return _m
