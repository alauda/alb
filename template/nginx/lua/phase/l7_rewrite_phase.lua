-- format:on
local string_lower = string.lower
local ngx = ngx
local ngx_var = ngx.var
local ngx_header = ngx.header
local ngx_re = ngx.re
local ngx_log = ngx.log
local actx = require "ctx.alb_ctx"

local upstream = require "match_engine.upstream"
local redirect = require "l7_redirect"

local rewrite_header = require "rewrite_header"
local subsystem = ngx.config.subsystem
local replace = require "replace_prefix_match"
local e = require "error"
local pm = require("plugins.core.plugin_manager")
local cors = require "cors"

local function set_cors(matched_policy)
    local enable_cors = matched_policy["enable_cors"]
    if enable_cors == true then
        if ngx.ctx.alb_ctx.var.method == 'OPTIONS' then
            cors.common_cors_header(matched_policy, ngx_header)
            ngx_header['Access-Control-Max-Age'] = '1728000'
            ngx_header['Content-Type'] = 'text/plain charset=UTF-8'
            ngx_header['Content-Length'] = '0'
            ngx.exit(ngx.HTTP_NO_CONTENT)
        end
    end
end

local function set_vhost(matched_policy)
    local vhost = matched_policy["vhost"]
    if vhost ~= "" then
        ngx_var.custom_host = vhost
    end
end

local function set_backend_protocol(matched_policy)
    local backend_protocol = matched_policy["backend_protocol"]
    if backend_protocol ~= "" then
        -- collaborate with proxy_pass $backend_protocol://http_backend; in nginx.conf
        ngx_var.backend_protocol = string_lower(backend_protocol)
    end
end

local function set_rewrite_url(matched_policy)
    local prefix_match = matched_policy["rewrite_prefix_match"]     -- notemptystring | nil
    local replace_prefix = matched_policy["rewrite_replace_prefix"] -- string | nil
    if prefix_match ~= nil and prefix_match ~= "" then
        ngx.req.set_uri(replace.replace(ngx.ctx.alb_ctx.var.uri, prefix_match, replace_prefix), false)
        return
    end

    local rewrite_target = matched_policy["rewrite_target"]
    local policy_url = matched_policy["rewrite_base"]
    local rewrite_base = matched_policy["rewrite_base"]
    if policy_url == "" then
        policy_url = matched_policy["url"]
    end
    if rewrite_target ~= "" then
        if policy_url == "" then
            policy_url = "/"
        end
        if rewrite_base == ".*" then
            ngx.req.set_uri(rewrite_target, false)
            return
        end
        local new_uri = ngx_re.sub(ngx.ctx.alb_ctx.var.uri, policy_url, rewrite_target, "jo")
        ngx.req.set_uri(new_uri, false)
    end
end

local function do_l7_rewrite()
    ngx.ctx.alb_ctx = actx.new()
    -- entrypoint of l7 req
    -- ngx_var.protocol is nil in http subsystem
    local t_upstream, matched_policy, errmsg = upstream.get_upstream(subsystem, "tcp", ngx_var.server_port)
    if errmsg ~= nil then
        ngx_log(ngx.ERR, errmsg)
        return e.exit_with_code(e.InvalidUpstream, tostring(errmsg), ngx.HTTP_NOT_FOUND)
    end

    if t_upstream == nil then
        local msg = "alb upstream not found"
        ngx_log(ngx.ERR, msg)
        return e.exit_with_code(e.InvalidUpstream, msg, ngx.HTTP_NOT_FOUND)
    end

    if matched_policy == nil then
        local msg = "alb policy not found"
        ngx_log(ngx.ERR, msg)
        return e.exit_with_code(e.InvalidUpstream, msg, ngx.HTTP_NOT_FOUND)
    end
    local to_location = matched_policy["to_location"] or ""
    local ami_in_root_location = ngx.var.location_mode == "root"

    if to_location ~= "" and ami_in_root_location then
        ngx_log(ngx.INFO, "exec to_location: " .. to_location)
        ngx.exec("@" .. to_location)
        return
    end

    ngx.ctx.matched_policy = matched_policy
    ngx.ctx.upstream = t_upstream
    ngx.ctx.alb_ctx.matched_policy = matched_policy

    -- redirect可以直接阻断后端的配置。比较特殊
    -- 所以需要先执行redirect，在处理其他的plugin
    local config = matched_policy.config or {}
    if config.redirect ~= nil then
        redirect.redirect(config.redirect)
        return
    end

    set_cors(matched_policy)
    set_backend_protocol(matched_policy)
    set_vhost(matched_policy)
    set_rewrite_url(matched_policy)
    rewrite_header.rewrite_request_header()
    pm.after_rule_match_hook(ngx.ctx.alb_ctx)
end

do_l7_rewrite()
