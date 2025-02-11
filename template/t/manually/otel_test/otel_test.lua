-- format:on
local _M = {}
local h = require "test-helper"
local u = require "util"
local ph = require("policy_helper")
local re_split = require("ngx.re").split
local sext = require("utils.string_ext")

function _M.as_backend()
    ngx.say "ok"
end

--[=====[
对于 policy.json来说 没有merge的问题 在每一个rule 上，它看到的就是一个完整的配置,要么有 要么没有
--]=====]

---@param otel_ref string|nil the otel ref use in each rule
---@return table policy the policy
local function policy_with_otel(otel_ref, custom)
    local otel_config = function (name)
        if custom and custom[name] then
            return { otel = custom[name] }
        end
        return { refs = { otel = otel_ref } }
    end

    --[=====[
        frontend-service-port    8080
        customer-service-port    8081
        driver-service-port      8082 # grpc we don't support yet
        route-service-port       8083
    --]=====]
    local default_collect_address = "http://127.0.0.1:4318"
    -- LuaFormatter off
    return {
        http = {
            tcp = {
                ["80"] = {
                    { rule = "test",     ingress = { name = "", path_index = 1, rule_index = 1 },  internal_dsl = { { "STARTS_WITH", "URL", "/test" } }, upstream = "test",                config = otel_config("2") },
                    { rule = "frontend", internal_dsl = { { "STARTS_WITH", "URL", "/dispatch" } }, upstream = "frontend",                                config = otel_config("frontend"), source_type = "ingress",  source_name = "ing-x", source_ns = "ing-x", ingress_rule_index = "1:1" },
                    { rule = "customer", internal_dsl = { { "STARTS_WITH", "URL", "/customer" } }, upstream = "customer",                                config = otel_config("customer") },
                    { rule = "router",   internal_dsl = { { "STARTS_WITH", "URL", "/route" } },    upstream = "router",                                  config = otel_config("router") }
                }
            }
        },
        config = {
            ["off_trace"] = {
                type = "otel",
                otel = {
                    exporter = { collector = { address = default_collect_address, request_timeout = 1000 }, batch_span_processor = { max_queue_size = 2048 } },
                    flags = { hide_upstream_attrs = false, trust_incoming_span = false },
                    sampler = { name = "always_off" },
                    resource = {}
                }
            },
            ["parent-base"] = {
                type = "otel",
                otel = {
                    exporter = { collector = { address = default_collect_address, request_timeout = 1000 } },
                    flags = { hide_upstream_attrs = false, trust_incoming_span = false },
                    sampler = {
                        name = "parent_base", options = { parent_name = "always_off" },
                    }
                }
            },
            ["ratio-base"] = {
                type = "otel",
                otel = {
                    exporter = { collector = { address = default_collect_address, request_timeout = 1000 } },
                    flags = { hide_upstream_attrs = false, trust_incoming_span = false },
                    sampler = {
                        name = "trace_id_ratio", options = { fraction = "0.5" }
                    }
                }
            },
            ["default_trace"] = {
                type = "otel",
                otel = {
                    exporter = { collector = { address = default_collect_address, request_timeout = 1000 }, batch_span_processor = { max_queue_size = 2048 } },
                    flags = { hide_upstream_attrs = false },
                    sampler = { name = "always_on" },
                    resource = { ["a"] = "x" }
                }
            }
        },
        backend_group = {
            { name = "frontend", mode = "http", backends = { { address = "127.0.0.1", port = 8080, weight = 100, svc = "frontend", ns = "default" } } },
            { name = "customer", mode = "http", backends = { { address = "127.0.0.1", port = 8081, weight = 100, svc = "customer", ns = "default" } } },
            { name = "router",   mode = "http", backends = { { address = "127.0.0.1", port = 8083, weight = 100, svc = "router", ns = "default" } } },
            { name = "test",     mode = "http", backends = { { address = "127.0.0.1", port = 1880, weight = 100, svc = "test", ns = "default" } } }
        }
    }
    -- LuaFormatter on
end

---@param policy table
---@return table trace_res
---@return string trace_id
function _M.test_trace(policy, opt)
    ph.set_policy_lua(policy)
    local curl_opt = {}
    if opt ~= nil and opt.traceparent ~= nil then
        curl_opt = { headers = { traceparent = opt.traceparent } }
    end
    local res = h.assert_curl("http://127.0.0.1:80/dispatch?customer=567&nonse=" .. tostring(ngx.now()), curl_opt)
    local trace = res.headers.Traceresponse
    local trace_ids, err = re_split(trace, "-", "jo")
    ---@cast trace_ids -nil
    h.assert_is_nil(err)
    local trace_id = trace_ids[2]
    ngx.sleep(10) -- it seems we need wait a little bit for the trace to be ready
    local trace_res = h.just_curl("http://127.0.0.1:16686/api/traces/" .. trace_id .. "?prettyPrint=true")
    -- u.logs("res", trace_res.body)
    -- u.logs("alb trace", alb_trace)
    -- u.logs("trace id", trace_id)
    return trace_res, trace_id
end

---@param policy table
---@param should_has_trace boolean
function _M.assert_has_trace(policy, should_has_trace)
    local trace_res, _ = _M.test_trace(policy)
    local alb_trace = sext.lines_grep(trace_res.body, "alb")
    if should_has_trace then
        h.assert_true(#alb_trace > 0, "should has alb trace " .. tostring(#alb_trace))
    else
        h.assert_true(#alb_trace == 0, "should has no alb trace")
    end
end

function _M.test_parent()
    local res, id = _M.test_trace(policy_with_otel("parent-base"),
        { traceparent = "00-a0000000000000010000000000000001-0000000000000001-01" })
    h.assert_eq(res.status, 200)
    u.logs("trace id", id)
    u.logs("res ", res.status)
    local res, id = _M.test_trace(policy_with_otel("parent-base"),
        { traceparent = "00-b0000000000000010000000000000001-0000000000000002-00" })
    h.assert_eq(res.status, 404)
    u.logs("trace id", id)
    u.logs("res ", res.status)
    local res, id = _M.test_trace(policy_with_otel("parent-base"))
    h.assert_eq(res.status, 404)
    u.logs("trace id", id)
    u.logs("res ", res.status)
end

function _M.test_ratio()
    -- restart jaeger each test..
    local res, id = _M.test_trace(policy_with_otel("ratio-base"),
        { traceparent = "00-00000000000000010000000000000001-0000000000000001-01" })
    u.logs("trace id", id)
    u.logs("res ", res.status)
    h.assert_eq(res.status, 200)
    local res, id = _M.test_trace(policy_with_otel("ratio-base"),
        { traceparent = "00-ffffffff000000000000000100000001-0000000000000001-01" })
    u.logs("trace id", id)
    u.logs("res ", res.status)
    h.assert_eq(res.status, 404)
end

function _M.test()
    _M.test_parent()
    _M.test_ratio()
    _M.assert_has_trace(policy_with_otel("default_trace"), true)
    _M.assert_has_trace(policy_with_otel(nil), false)
    _M.assert_has_trace(policy_with_otel("default_trace"), true)
    _M.assert_has_trace(policy_with_otel(nil), false)
    -- -- off_trace should not have trace
    _M.assert_has_trace(policy_with_otel("off_trace"), false)
    -- -- we could disable trace for a specific rule
    _M.assert_has_trace(policy_with_otel("default_trace", { router = { otel_ref = nil } }), true)
end

return _M
