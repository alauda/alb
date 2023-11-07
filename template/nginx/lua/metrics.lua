-- format:on
local _M = {}

local table_clear = require("table.clear")
local prometheus = require "resty.prometheus"
local tonumber = tonumber
local ngx = ngx
local ngx_var = ngx.var
local ngx_log = ngx.log

local _metrics = {}
local _prometheus

function _M.init()
    ngx_log(ngx.INFO, "init metrics")
    table_clear(_metrics)
    _prometheus = prometheus.init("prometheus_metrics")
    _metrics.requests = _prometheus:counter("nginx_http_requests", "Number of HTTP requests", {"port", "rule"})
    _metrics.mismatch_rule_requests = _prometheus:counter("nginx_http_mismatch_rule_requests", "Number of mistach rule requests", {"port"})
    _metrics.status = _prometheus:counter("nginx_http_status", "HTTP status code per rule", {"port", "rule", "status"})
    _metrics.request_sizes = _prometheus:counter("nginx_http_request_size_bytes", "Size of HTTP requests", {"port"})
    _metrics.response_sizes = _prometheus:counter("nginx_http_response_size_bytes", "Size of HTTP responses", {"port"})

    _metrics.upstream_requests = _prometheus:counter("nginx_http_upstream_requests", "Number of HTTP requests per upstream", {"port", "rule", "upstream_ip"})

    _metrics.upstream_requests_status = _prometheus:counter("nginx_http_upstream_requests_status", "HTTP status code per rule per upstream", {"port", "rule", "upstream_ip", "status"})

    _metrics.alb_error = _prometheus:counter("alb_error", "error cause by alb itself", {"port"})
end

function _M.log()
    local rule_name = ngx_var.rule_name
    if rule_name and rule_name ~= "" then
        _metrics.requests:inc(1, ngx_var.server_port, rule_name)
        _metrics.status:inc(1, ngx_var.server_port, rule_name, ngx_var.status)
        _metrics.request_sizes:inc(tonumber(ngx_var.request_length), ngx_var.server_port)
        _metrics.response_sizes:inc(tonumber(ngx_var.bytes_sent), ngx_var.server_port)

        _metrics.upstream_requests:inc(1, ngx_var.server_port, rule_name, ngx_var.upstream_addr or "")
        _metrics.upstream_requests_status:inc(1, ngx_var.server_port, rule_name, ngx_var.upstream_addr or "", ngx_var.status)
    else
        _metrics.requests:inc(1, ngx_var.server_port, "")
        _metrics.mismatch_rule_requests:inc(1, ngx_var.server_port)
    end
    if ngx.ctx.is_alb_err == true then
        _metrics.alb_error:inc(1, ngx_var.server_port)
    end
end

function _M.collect()
    _prometheus:collect()
end

function _M.clear()
    _prometheus:clear()
    ngx_log(ngx.INFO, "clear prometheus metrics finished")
end

return _M
