local _m = {}
local u = require("util")
local ph = require("policy_helper")
local subsys = require "utils.subsystem"
local h = require "test-helper"


function _m.as_backend()
    u.logs("as_backend", ngx.var.arg_sleep)
    ngx.say("ok")
    if ngx.var.arg_half then
        ngx.flush(false)
    end
    if ngx.var.arg_sleep then
        local sleep = tonumber(ngx.var.arg_sleep)
        u.logs("do sleep", sleep)
        ngx.sleep(sleep)
    end
    u.logs("ret")
    ngx.say("over")
end

function _m.test()
    if subsys.is_http_subsystem() then
        _m.test_l7()
    end
    if subsys.is_stream_subsystem() then
        _m.test_l4()
    end
end

---@type NgxPolicy
local policy = {
    certificate_map = {},
    stream = {
        tcp = {
            [81] = {
                {
                    plugins = { "timeout" },
                    rule = "1",
                    internal_dsl = {},
                    config = {
                        timeout = {
                            proxy_connect_timeout_ms = 2500,
                            proxy_send_timeout_ms = 2500,
                            proxy_read_timeout_ms = 2500,
                        },
                    },
                    upstream = "u1"
                },
            }
        }
    },
    http = {
        tcp = {
            [80] = {
                { plugins = { "timeout" }, rule = "1", internal_dsl = { { "STARTS_WITH", "URL", "/" } }, config = { refs = { timeout = "timeout-1" } }, upstream = "u1" },
            }
        }
    },
    config = {
        ["timeout-1"] = {
            type = "timeout",
            timeout = {
                proxy_connect_timeout_ms = 2500,
                proxy_send_timeout_ms = 2500,
                proxy_read_timeout_ms = 2500,
            }
        },
    },
    backend_group = { { name = "u1", mode = "http", backends = { { address = "127.0.0.1", port = 1880, weight = 100 } }, } }
}

function _m.test_l4()
    ph.set_policy_lua(policy)
    u.logs("should timeout")
    ngx.update_time()
    local s = ngx.time()
    local res, err = u.curl("http://127.0.0.1:81?sleep=5")
    ngx.update_time()
    local e = ngx.time()
    local spend = (e - s)
    u.logs("timeout via alb", res, err)
    h.assert_true(spend > 1 and spend < 5)

    u.logs("should ok")
    ngx.update_time()
    local s = ngx.time()
    local res, err = u.curl("http://127.0.0.1:81?sleep=1")
    h.assert_eq(res.status, 200)
    ngx.update_time()
    local e = ngx.time()
    local spend = (e - s)
    h.assert_true(spend < 5)
    u.logs("timeout via alb", res, err)
end

-- timeout有两种情况
-- 1. upstream发了一部分数据回来。这时不会重试。client不会收到504
--  - client会收到不完整的http请求
--  - 在超时时nginx会直接发送fin报文给upstream和client。不会等upstream。
-- 2. upstream没有发数据回来。这时会重试(重复发http请求)，client会收到504. 受 proxy_next_upstream 控制。

function _m.test_l7()
    ph.set_policy_lua(policy)
    u.logs("should not retry")
    ngx.update_time()
    local s = ngx.time()
    local res, err = u.curl("http://127.0.0.1:80?sleep=5&&half=true")
    ngx.update_time()
    local e = ngx.time()
    local spend = (e - s)
    u.logs("timeout via alb", subsys.CURRENT_SYBSYSTEM, res, err, s, e, (e - s))
    h.assert_true(spend > 1 and spend < 5)

    u.logs("should retry when get")
    ngx.update_time()
    local s = ngx.time()
    local res, err = u.curl("http://127.0.0.1:80?sleep=5")
    ngx.update_time()
    local e = ngx.time()
    u.logs("timeout via alb", subsys.CURRENT_SYBSYSTEM, res, err, s, e, (e - s))
    h.assert_eq(res.status, 504)

    u.logs("should not retry when post")
    ngx.update_time()
    local s = ngx.time()
    local res, err = u.curl("http://127.0.0.1:80?sleep=5", { method = "POST" })
    ngx.update_time()
    local e = ngx.time()
    u.logs("timeout via alb", subsys.CURRENT_SYBSYSTEM, res, err, s, e, (e - s))
    h.assert_eq(res.status, 504)
    local spend = (e - s)
    h.assert_true(spend > 1 and spend < 5)

    u.logs("should ok")
    ngx.update_time()
    local s = ngx.time()
    local res, err = u.curl("http://127.0.0.1:80?sleep=1")
    ngx.update_time()
    local e = ngx.time()
    u.logs("timeout via alb", subsys.CURRENT_SYBSYSTEM, res, err, s, e, (e - s))
    h.assert_eq(res.status, 200)
    local spend = (e - s)
    h.assert_true(spend < 3)
end

return _m
