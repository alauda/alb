-- format:on
local e = require "error"
local str = require "resty.string"
local pm = require("plugins.core.plugin_manager")

local matched_policy = ngx.ctx.matched_policy
if matched_policy == nil then
    return
end
local cors = require "cors"
local rewrite_header = require "rewrite_header"

cors.header_filter()
rewrite_header.rewrite_response_header()
if ngx.ctx.is_alb_err then
    return
end

if ngx.ctx.alb_ctx.matched_policy then
    pm.response_header_filter_hook(ngx.ctx.alb_ctx)
end

local code = str.atoi(ngx.var.status)
if code >= 400 then
    e.http_backend_error(code, "read " .. tostring(ngx.var.upstream_bytes_received) .. " byte data from backend")
end
