local string_lower = string.lower
local ngx = ngx
local ngx_var = ngx.var
local ngx_header = ngx.header
local ngx_re = ngx.re
local ngx_log = ngx.log
local ngx_exit = ngx.exit

local upstream = require "upstream"
local var_proxy = require "var_proxy"
local redirect = require "l7_redirect"

local subsystem = ngx.config.subsystem
ngx.ctx.alb_ctx = var_proxy.new()

-- ngx_var.protocol is nil in http subsystem
local t_upstream, matched_policy, errmsg = upstream.get_upstream(subsystem, "tcp", ngx_var.server_port)
if t_upstream ~= nil then
    ngx_var.upstream = t_upstream
end

if matched_policy ~= nil then
    ngx.ctx.matched_policy = matched_policy
	if redirect.need() then
		redirect.redirect()
		return -- unreachable!()
	end

    ngx_var.rule_name = matched_policy["rule"]
    local enable_cors = matched_policy["enable_cors"]
    if enable_cors == true then
        if ngx.ctx.alb_ctx.method == 'OPTIONS' then
            if matched_policy["cors_allow_origin"] ~= "" then
                ngx_header['Access-Control-Allow-Origin'] = matched_policy["cors_allow_origin"]
            else
                ngx_header['Access-Control-Allow-Origin'] = "*"
            end
            ngx_header['Access-Control-Allow-Credentials'] = 'true'
            ngx_header['Access-Control-Allow-Methods'] = 'GET, PUT, POST, DELETE, PATCH, OPTIONS'
            if matched_policy["cors_allow_headers"] ~= "" then
                ngx_header['Access-Control-Allow-Headers'] = matched_policy["cors_allow_headers"]
            else
                ngx_header['Access-Control-Allow-Headers'] = 'DNT,X-CustomHeader,Keep-Alive,User-Agent,X-Requested-With,If-Modified-Since,Cache-Control,Content-Type,Authorization'
            end
            ngx_header['Access-Control-Max-Age'] = '1728000'
            ngx_header['Content-Type'] = 'text/plain charset=UTF-8'
            ngx_header['Content-Length'] = '0'
            ngx_exit(ngx.HTTP_NO_CONTENT)
        end
    end
    local backend_protocol = matched_policy["backend_protocol"]
    if backend_protocol ~= "" then
        -- collaborate with proxy_pass $backend_protocol://http_backend; in nginx.conf
        ngx_var.backend_protocol = string_lower(backend_protocol)
    end
    local vhost = matched_policy["vhost"]
    if vhost ~= "" then
        ngx_var.custom_host = vhost
    end
    local rewrite_target = matched_policy["rewrite_target"]
    local policy_url = matched_policy["rewrite_base"]
    if policy_url == "" then
        policy_url = matched_policy["url"]
    end
    if rewrite_target ~= "" then
        if policy_url == "" then
            policy_url = "/"
        end
        local new_uri = ngx_re.sub(ngx.ctx.alb_ctx.uri, policy_url, rewrite_target, "jo")
        ngx.req.set_uri(new_uri, false)
    end
elseif errmsg ~= nil then
    ngx_log(ngx.ERR, errmsg)
    ngx_exit(ngx.HTTP_NOT_FOUND)
end
