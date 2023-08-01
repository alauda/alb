local s_ext = require "utils.string_ext"
local g_ext = require "utils.generic_ext"
local replace = require "replace_prefix_match"

local _M = {}

function _M.redirect()
    local ngx_redirect = ngx.redirect
    local policy = ngx.ctx.matched_policy

    local redirect_scheme = policy["redirect_scheme"]
    local redirect_host = policy["redirect_host"]
    local redirect_port = policy["redirect_port"]
    local redirect_url = policy["redirect_url"]
    local prefix_match = policy["redirect_prefix_match"] -- notemptystring | nil
    local replace_prefix = policy["redirect_replace_prefix"] -- string | nil
    local redirect_code = g_ext.nil_or(policy["redirect_code"], 302, 0)

    -- fast path
    if redirect_scheme == nil and redirect_host == nil and redirect_port == nil then
        if redirect_url ~= "" then
            ngx_redirect(redirect_url, redirect_code)
            return -- unreachable!{}
        end
    end

    local scheme = s_ext.nil_or(redirect_scheme, ngx.ctx.alb_ctx.scheme)
    local host = s_ext.nil_or(redirect_host, ngx.ctx.alb_ctx.host)
    local url = s_ext.nil_or(redirect_url, ngx.ctx.alb_ctx.uri)
    if prefix_match ~= nil and prefix_match ~= "" then
        replace_prefix = s_ext.nil_or(replace_prefix, "")
        url = replace.replace(url, prefix_match, replace_prefix)
    end

    redirect_url = scheme .. "://" .. host .. url
    if redirect_port ~= nil then
        redirect_url = scheme .. "://" .. host .. ":" .. tostring(redirect_port) .. url
    end
    ngx_redirect(redirect_url, redirect_code)
end

--- check is this request matched rule need redirect
-- @return bool true if need redirect.
function _M.need()
    local policy = ngx.ctx.matched_policy
    if policy == nil then
        return false
    end
    local redirect_url = policy["redirect_url"] -- notemptystring | "" | nil
    local prefix_match = policy["redirect_prefix_match"] -- notemptystring | nil
    local redirect_code = policy["redirect_code"] -- notzeroint 0 | 0 |nil
    local redirect_scheme = policy["redirect_scheme"] -- notemptystring | nil
    local redirect_host = policy["redirect_host"] -- notemptystring | nil
    local redirect_port = policy["redirect_port"] -- notemptystring | nil
    if redirect_url ~= nil and redirect_url ~= "" then
        return true
    end
    if prefix_match ~= nil and prefix_match ~= "" then
        return true
    end
    if redirect_code ~= nil and redirect_code ~= 0 then
        return true
    end
    if redirect_scheme ~= nil then
        return true
    end
    if redirect_host ~= nil then
        return true
    end
    if redirect_port ~= nil then
        return true
    end
    return false
end

return _M
