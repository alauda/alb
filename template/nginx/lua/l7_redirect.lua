local s_ext = require "utils.string_ext"
local g_ext = require "utils.generic_ext"
local replace = require "replace_prefix_match"
local ngx_redirect = ngx.redirect

local _M = {}
---@param policy RedirectCr
function _M.redirect(policy)
    local redirect_scheme = policy.scheme
    local redirect_host = policy.host
    local redirect_port = policy.port
    local redirect_url = policy.url
    local prefix_match = policy.prefix_match     -- notemptystring | nil
    local replace_prefix = policy.replace_prefix -- string | nil
    local redirect_code = g_ext.nil_or(policy.code, 302, 0)

    -- fast path
    if redirect_scheme == nil and redirect_host == nil and redirect_port == nil then
        if not s_ext.is_nill(redirect_url) then
            ngx_redirect(redirect_url, redirect_code)
            return -- unreachable!{}
        end
    end

    local scheme = s_ext.nil_or(redirect_scheme, ngx.ctx.alb_ctx.var.scheme)
    local host = s_ext.nil_or(redirect_host, ngx.ctx.alb_ctx.var.host)
    local url = s_ext.nil_or(redirect_url, ngx.ctx.alb_ctx.var.uri)
    if not s_ext.is_nill(prefix_match) then
        replace_prefix = s_ext.nil_or(replace_prefix, "")
        url = replace.replace(url, prefix_match, replace_prefix)
    end

    redirect_url = scheme .. "://" .. host .. url
    if redirect_port ~= nil then
        redirect_url = scheme .. "://" .. host .. ":" .. tostring(redirect_port) .. url
    end
    ngx_redirect(redirect_url, redirect_code)
end

return _M
