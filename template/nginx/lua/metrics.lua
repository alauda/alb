-- format:on
local _M = {}

local table_clear = require("table.clear")
local prometheus = require "prometheus.prometheus"
local actx = require "ctx.alb_ctx"
local common = require("utils.common")
local mauth = require("metrics_auth")

local tonumber = tonumber
local ngx = ngx
local ngx_var = ngx.var
local ngx_log = ngx.log

---@type { [string]: table|nil }
local _metrics = {}
local _prometheus


local ALB_NS = os.getenv("ALB_NS") or ""

function _M.init()
    ngx_log(ngx.INFO, "init metrics for " .. tostring(ngx.worker.id()) .. " " .. tostring(ngx.worker.pid()))
    table_clear(_metrics)
    -- LuaFormatter off
    _prometheus = prometheus.init("prometheus_metrics")

    _metrics.connection = _prometheus:gauge("nginx_http_connections", "Number of HTTP connections", { "state" })

    _metrics.mismatch_rule_requests = _prometheus:counter(
        "nginx_http_mismatch_rule_requests",
        "Number of mismatch rule requests",
        { "port", "method" }
    )

    _metrics.status = _prometheus:counter(
        "nginx_http_status",
        "HTTP status code per rule",
        { "port", "rule", "status", "method", "source_type", "source_namespace", "source_name", "svc_ns", "svc_name" }
    )
    _metrics.request_sizes = _prometheus:counter(
        "nginx_http_request_size_bytes",
        "Size of HTTP requests",
        { "port", "rule", "status", "method", "source_type", "source_namespace", "source_name", "svc_ns", "svc_name" }
    )
    _metrics.response_sizes = _prometheus:counter(
        "nginx_http_response_size_bytes",
        "Size of HTTP responses",
        { "port", "rule", "status", "method", "source_type", "source_namespace", "source_name", "svc_ns", "svc_name" }
    )
    _metrics.latency = _prometheus:histogram(
        "nginx_http_request_duration_seconds",
        "HTTP request latency",
        { "port", "rule", "source_type", "source_namespace", "source_name", "svc_ns", "svc_name" },
        { .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10 }
    )

    _metrics.upstream_requests_status = _prometheus:counter(
        "nginx_http_upstream_requests_status",
        "HTTP status code per rule per upstream",
        { "port", "rule", "upstream_ip", "status", "method", "source_type", "source_namespace", "source_name", "svc_ns",
            "svc_name" }
    )
    _metrics.upstream_latency = _prometheus:histogram(
        "nginx_http_upstream_request_duration_seconds",
        "HTTP request latency per upstream",
        { "port", "rule", "upstream_ip", "source_type", "source_namespace", "source_name", "svc_ns", "svc_name" },
        { .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10 }
    )

    _metrics.alb_error = _prometheus:counter("alb_error", "error cause by alb itself", { "port" })
    _metrics.metrics_free_cache_size = _prometheus:gauge("metrics_cache_size", "size of metrics cache")

    _metrics.shared_dict_capacity = _prometheus:gauge("shared_dict_capacity", "capacity of shared dict", { "name" })
    _metrics.shared_dict_free = _prometheus:gauge("shared_dict_free", "free space of shared dict", { "name" })
    _metrics.alb_loop_time_ms = _prometheus:gauge("alb_loop_time", "each loop time of alb")

    -- LuaFormatter on
end

function _M.log()
    ---@type Policy
    local policy = ngx.ctx.matched_policy
    local rule_name = policy and policy.rule or ""
    local server_port = ngx_var.server_port or ""
    local status = ngx_var.status or ""
    local request_method = ngx_var.request_method or ""
    local request_length = tonumber(ngx_var.request_length) or 0
    local request_time = tonumber(ngx_var.request_time) or 0
    local bytes_sent = tonumber(ngx_var.bytes_sent) or 0
    local upstream_addr = ngx_var.upstream_addr or ""
    local upstream_response_time = tonumber(ngx_var.upstream_response_time) or 0
    local source = policy and policy.source or {}
    -- 如果没有source,默认是rule
    local source_type = source.source_type or "rule"
    local source_namespace = source.source_ns or ALB_NS
    local source_name = source.source_name or rule_name
    local svc_ns, svc_name = actx.get_peer_svc_ns_name()

    _metrics.status:inc(1,
        { server_port, rule_name, status, request_method, source_type, source_namespace, source_name, svc_ns, svc_name })
    _metrics.request_sizes:inc(request_length,
        { server_port, rule_name, status, request_method, source_type, source_namespace, source_name, svc_ns, svc_name })
    _metrics.response_sizes:inc(bytes_sent,
        { server_port, rule_name, status, request_method, source_type, source_namespace, source_name, svc_ns, svc_name })
    _metrics.latency:observe(request_time,
        { server_port, rule_name, source_type, source_namespace, source_name, svc_ns, svc_name })

    if rule_name ~= "" then
        _metrics.upstream_requests_status:inc(1,
            { server_port, rule_name, upstream_addr, status, request_method, source_type, source_namespace, source_name,
                svc_ns, svc_name })
        _metrics.upstream_latency:observe(upstream_response_time,
            { server_port, rule_name, upstream_addr, source_type, source_namespace, source_name, svc_ns, svc_name })
    else
        _metrics.mismatch_rule_requests:inc(1, { server_port, request_method })
    end
    if ngx.ctx.is_alb_err == true then
        _metrics.alb_error:inc(1, { server_port })
    end
end

local DISABLE_ALB_LOOP_TIME_METRICS = os.getenv("DISABLE_ALB_LOOP_TIME_METRICS") == "true"

function _M.collect()
    mauth.verify_auth()
    _metrics.connection:set(ngx_var.connections_reading, { "reading" })
    _metrics.connection:set(ngx_var.connections_waiting, { "waiting" })
    _metrics.connection:set(ngx_var.connections_writing, { "writing" })
    _metrics.connection:set(ngx_var.connections_active, { "active" })
    _metrics.metrics_free_cache_size:set(ngx.shared.prometheus_metrics:free_space())

    for name, dict in pairs(ngx.shared) do
        _metrics.shared_dict_capacity:set(dict:capacity(), { name })
        _metrics.shared_dict_free:set(dict:free_space(), { name })
    end
    if not DISABLE_ALB_LOOP_TIME_METRICS then
        local alb_loop_time = _M.get_alb_loop_time()
        _metrics.alb_loop_time_ms:set(alb_loop_time, {})
    end

    _prometheus:collect()

    if ngx.shared.prometheus_metrics:free_space() < ngx.shared.prometheus_metrics:capacity() * 0.15 then
        ngx_log(ngx.WARN, "outof prometheus metrics memory")
        _M.clear()
    end
end

function _M.clear()
    mauth.verify_auth()
    for name, metrics in pairs(_metrics) do
        ngx_log(ngx.INFO, "clean prometheus metrics: ", name)
        metrics:reset()
    end
    ngx_log(ngx.INFO, "clear prometheus metrics finished")
end

local ALB_LOOP_TIME_PATH = os.getenv("ALB_LOOP_TIME_PATH") or "/etc/alb2/nginx/alb_loop_time"
function _M.get_alb_loop_time()
    local alb_loop_time, err = common.file_read_to_string(ALB_LOOP_TIME_PATH)
    if err or alb_loop_time == nil or alb_loop_time == "" then
        return 0
    end
    local alb_loop_time = tonumber(alb_loop_time)
    if alb_loop_time then
        return alb_loop_time
    end
    return 0
end

return _M
