-- format:on
local ngx = ngx
local ngx_var = ngx.var
local ngx_log = ngx.log
local e = require("error")

local upstream = require "upstream"
local var_proxy = require "var_proxy"

local subsystem = ngx.config.subsystem
ngx.ctx.alb_ctx = var_proxy.new()

local t_upstream, matched_policy, errmsg = upstream.get_upstream(subsystem, ngx_var.protocol, ngx_var.server_port)
if t_upstream ~= nil then
    ngx.ctx.upstream = t_upstream
end

ngx.ctx.matched_policy = matched_policy

if errmsg ~= nil then
    ngx_log(ngx.ERR, errmsg)
    e.exit(e.InvalidUpstream, errmsg)
end
