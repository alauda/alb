local _M = {}

local table_clear  = require("table.clear"),
local prometheus = require "resty.prometheus"
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
    _metrics.status = _prometheus:counter(
      "nginx_http_status", "HTTP status code per rule", {"rule", "status"})
    _metrics.latency = _prometheus:histogram(
      "nginx_http_request_duration_seconds", "HTTP request latency", {"rule"})
    _metrics.request_sizes = _prometheus:histogram(
      "nginx_http_request_size_bytes", "Size of HTTP requests", nil,
      {10,100,1000,10000,100000,1000000})
    _metrics.response_sizes = _prometheus:histogram(
      "nginx_http_response_size_bytes", "Size of HTTP responses", nil,
      {10,100,1000,10000,100000,1000000})
end

function _M.log()
    _metrics.requests:inc(1)
    local rule_name = ngx_var.rule_name
    if rule_name and rule_name ~= "" then
      _metrics.status:inc(1, rule_name, ngx_var.status)
      local latency = (ngx.now() - ngx.req.start_time()) * 1000
      _metrics.latency:observe(latency, rule_name)
      _metrics.request_sizes:observe(ngx_var.request_sizes)
      _metrics.response_sizes:observe(ngx_var.bytes_sent)
    end
end

function _M.collect()
   _prometheus:collect()
end

return _M