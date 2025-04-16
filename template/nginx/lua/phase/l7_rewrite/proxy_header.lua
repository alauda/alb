local _M = {}
local actx = require("ctx.alb_ctx")
local cache = require("config.cache")
local x_forward = require("utils.x_forward")

local DISABLE_EXTRA_PROXY_HEADERS = os.getenv("DISABLE_EXTRA_PROXY_HEADERS") == "true"
--- comment
--- @param ctx AlbCtx
function _M.set_proxy_header(ctx)
    _M.init_forwarded_for_var(actx.get_ngx_var(ctx, "http_x_forwarded_for"))
    local use_forwarded_headers = _M.use_forwarded_headers(ctx)
    if not use_forwarded_headers then
        ngx.req.set_header("X-Forwarded-For", actx.get_ngx_var(ctx, "proxy_add_x_forwarded_for"))
        ngx.req.set_header("X-Forwarded-Proto", actx.get_ngx_var(ctx, "scheme"))     -- client_request_scheme
        ngx.req.set_header("X-Forwarded-Host", actx.get_ngx_var(ctx, "http_host"))   -- client_request_host
        ngx.req.set_header("X-Forwarded-Port", actx.get_ngx_var(ctx, "server_port")) -- alb_port
    end
    if not DISABLE_EXTRA_PROXY_HEADERS then
        ngx.req.set_header("X-REAL-IP", actx.get_ngx_var(ctx, "remote_addr")) -- remote_addr
        ngx.req.set_header("X-Original-URI", actx.get_ngx_var(ctx, "request_uri"))
        ngx.req.set_header("X-Scheme", actx.get_ngx_var(ctx, "scheme"))
        ngx.req.set_header("X-Original-URL",
            actx.get_ngx_var(ctx, "scheme") ..
            "://" .. actx.get_ngx_var(ctx, "http_host") .. actx.get_ngx_var(ctx, "request_uri"))
        ngx.req.set_header("X-Original-Method", actx.get_ngx_var(ctx, "request_method"))
    end
end

--- @param ctx AlbCtx
--- @return boolean
function _M.use_forwarded_headers(ctx)
    local cfg = cache.get_config_from_policy(ctx.matched_policy, "proxy_header")
    local default = false
    if cfg == nil or cfg.use_forwarded_headers == nil then
        return default
    end
    return cfg.use_forwarded_headers
end

--- @param x_forwarded_for string
function _M.init_forwarded_for_var(x_forwarded_for)
    local first_forward = x_forward.parse_first_forward(x_forwarded_for)
    if first_forward ~= nil then
        ngx.var.first_forward_or_remote_addr = first_forward
        ngx.var.first_forward = first_forward
    end
end

return _M
