local _M = {}

local table_clear  = require("table.clear")
local prometheus = require "resty.prometheus"
local tonumber = tonumber
local ngx = ngx
local ngx_var = ngx.var
local ngx_log = ngx.log

local _metrics = {}
local _prometheus

function _M.init()
    ngx_log(ngx.ERR, "init metrics")
    table_clear(_metrics)
    _prometheus = prometheus.init("prometheus_metrics")
    _metrics.requests = _prometheus:counter(
      "nginx_http_requests_total", "Number of HTTP requests")
    _metrics.mismatch_rule_requests = _prometheus:counter(
      "nginx_http_mismatch_rule_requests_total", "Number of mistach rule requests", {"port"})
    _metrics.status = _prometheus:counter(
      "nginx_http_status", "HTTP status code per rule", {"rule", "status"})
    _metrics.request_sizes = _prometheus:counter(
      "nginx_http_request_size_bytes", "Size of HTTP requests")
    _metrics.response_sizes = _prometheus:counter(
      "nginx_http_response_size_bytes", "Size of HTTP responses")
end

function _M.log()
    _metrics.requests:inc(1)
    local rule_name = ngx_var.rule_name
    if rule_name and rule_name ~= "" then
      _metrics.status:inc(1, rule_name, ngx_var.status)
      _metrics.request_sizes:inc(tonumber(ngx_var.request_length))
      _metrics.response_sizes:inc(tonumber(ngx_var.bytes_sent))
    else
      _metrics.mismatch_rule_requests:inc(1, ngx_var.server_port)
    end
end

function _M.collect()
   _prometheus:collect()
end

return _M