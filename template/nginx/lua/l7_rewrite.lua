local string_lower = string.lower
local ngx_var = ngx.var
local ngx_ctx = ngx.ctx
local ngx_header = ngx.header
local ngx_re = ngx.re
local ngx_req = ngx.req
local ngx_log = ngx.log
local ngx_exit = ngx.exit
local ngx_redirect = ngx.redirect

local resty_var = require("resty.ngxvar")
local upstream = require "upstream"

ngx_ctx.var_req = resty_var.request()

local t_upstream, matched_policy, errmsg = upstream.get_upstream(ngx_var.server_port)
if t_upstream ~= nil then
  ngx_var.upstream = t_upstream
end
if matched_policy ~= nil then
  local redirect_url = matched_policy["redirect_url"]
  local redirect_code = matched_policy["redirect_code"]
  if redirect_url ~= "" then
    ngx_redirect(redirect_url, redirect_code)
  end
  ngx_var.rule_name = matched_policy["rule"]
  local enable_cors = matched_policy["enable_cors"]
  if enable_cors == true then
    if ngx_req.get_method() == 'OPTIONS' then
      ngx_header['Access-Control-Allow-Origin'] = '*'
      ngx_header['Access-Control-Allow-Credentials'] = 'true'
      ngx_header['Access-Control-Allow-Methods'] = 'GET, PUT, POST, DELETE, PATCH, OPTIONS'
      ngx_header['Access-Control-Allow-Headers'] = 'DNT,X-CustomHeader,Keep-Alive,User-Agent,X-Requested-With,If-Modified-Since,Cache-Control,Content-Type,Authorization'
      ngx_header['Access-Control-Max-Age'] = '1728000'
      ngx_header['Content-Type'] = 'text/plain charset=UTF-8'
      ngx_header['Content-Length'] = '0'
      ngx_exit(ngx.HTTP_NO_CONTENT)
    else
      ngx_header['Access-Control-Allow-Origin']= '*'
      ngx_header['Access-Control-Allow-Credentials'] = 'true'
      ngx_header['Access-Control-Allow-Methods'] = 'GET, PUT, POST, DELETE, PATCH, OPTIONS'
      ngx_header['Access-Control-Allow-Headers'] = 'DNT,X-CustomHeader,Keep-Alive,User-Agent,X-Requested-With,If-Modified-Since,Cache-Control,Content-Type,Authorization'
    end
  end
  local backend_protocol = matched_policy["backend_protocol"]
  if backend_protocol ~= "" then
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
    local new_uri = ngx_re.sub(ngx_var.uri, policy_url, rewrite_target, "jo")
    ngx_req.set_uri(new_uri, false)
  end
elseif errmsg ~= nil then
  ngx_log(ngx.ERR, errmsg)
  ngx_exit(ngx.HTTP_NOT_FOUND)
end
