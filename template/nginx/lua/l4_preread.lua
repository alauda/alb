-- format:on
local ngx = ngx
local ngx_var = ngx.var
local ngx_log = ngx.log
local e = require("error")

local upstream = require "match_engine.upstream"
local albctx = require "ctx.alb_ctx"

local subsystem = ngx.config.subsystem
ngx.ctx.alb_ctx = albctx.new()
local port = tonumber(ngx_var.server_port)
if port == nil then
    local msg = "invalid server_port " .. tostring(ngx_var.server_port)
    return e.exit(e.InvalidUpstream, msg)
end

local t_upstream, matched_policy, errmsg = upstream.get_upstream(subsystem, ngx_var.protocol, port)

if t_upstream == nil then
    local msg = "alb upstream not found "
    return e.exit(e.InvalidUpstream, msg)
end

if matched_policy == nil then
    local msg = "alb policy not found"
    return e.exit(e.InvalidUpstream, msg)
end

if t_upstream ~= nil then
    ngx.ctx.upstream = t_upstream
end

ngx.ctx.matched_policy = matched_policy
ngx.ctx.alb_ctx.matched_policy = matched_policy

if errmsg ~= nil then
    ngx_log(ngx.ERR, errmsg)
    e.exit(e.InvalidUpstream, errmsg)
end
