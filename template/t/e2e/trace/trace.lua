-- format:on
local _M = {}
local F = require "F"
local u = require "util"
local h = require "test-helper"
local common = require "utils.common"
local dsl = require "match_engine.dsl"
local ups = require "match_engine.upstream"
-- LuaFormatter off
local function default_policy()
    return {
        certificate_map = {},
        http = {
            tcp = {
                ["80"] = {
                    { rule = "1",      internal_dsl = { { "STARTS_WITH", "URL", "/t1" } },     upstream = "test-upstream-1" },
                    { rule = "2",      internal_dsl = { { "STARTS_WITH", "URL", "/t2" } },     upstream = "test-upstream-1" },
                    { rule = "3",      internal_dsl = { { "STARTS_WITH", "URL", "/t3" } },     upstream = "test-upstream-3" },
                    { rule = "4",      internal_dsl = { { "STARTS_WITH", "URL", "/t4" } },     upstream = "test-upstream-4" },
                    { rule = "5",      internal_dsl = { { "STARTS_WITH", "URL", "/t5" } },     upstream = "test-upstream-5" },
                    { rule = "t6",     internal_dsl = { { "REGEX", "URL", "/t6/*" } },         upstream = "test-upstream-1" },
                    { rule = "t7",     internal_dsl = { { "REGEX", "URL", "/t7/.*" } },        upstream = "test-upstream-1" },
                    { rule = "trace1", internal_dsl = { { "STARTS_WITH", "URL", "/trace1" } }, rewrite_prefix_match = "/trace1", rewrite_replace_prefix = "/trace2", upstream = "trace1" },
                    { rule = "trace2", internal_dsl = { { "STARTS_WITH", "URL", "/trace2" } }, upstream = "trace2" },
                }
            }
        },
        stream = { tcp = { ["81"] = { { upstream = "test-upstream-1" } } } },
        backend_group = {
            {
                name = "test-upstream-1",
                mode = "http",
                backends = {
                    { address = "127.0.0.1", port = 1880, weight = 100 }
                }
            },
            {
                name = "test-upstream-4", mode = "http", backends = {}
            },
            {
                name = "test-upstream-5",
                mode = "http",
                backends = {
                    { address = "127.0.0.1", port = 11880, weight = 100 } -- port not exist will connect error
                }
            },
            {
                name = "trace1",
                mode = "http",
                backends = {
                    { address = "127.0.0.1", port = 80, weight = 100 }
                }
            },
            {
                name = "trace2",
                mode = "http",
                backends = {
                    { address = "127.0.0.1", port = 1880, weight = 100 }
                }
            }
        }
    }
end
-- LuaFormatter on

function _M.as_backend()
    if string.find(ngx.var.uri, "404") then
        u.logs("expect 404 ====", ngx.var.uri)
        ngx.exit(404)
    end
    if string.find(ngx.var.uri, "500") then
        u.logs("expect 500 ====", ngx.var.uri)
        ngx.exit(500)
    end
    if string.find(ngx.var.uri, "timeout") then
        ngx.exit(504)
    end
    if string.find(ngx.var.uri, "sleep") then
        local t = tonumber(ngx.var.arg_sleep)
        u.logs("sleep in backend start", t)
        ngx.sleep(t)
        u.logs("sleep in backend over", t)
    end
    if string.find(ngx.var.uri, "detail") then
        local h, err = ngx.req.get_headers()
        if err ~= nil then
            ngx.say("err: " .. tostring(err))
        end
        for k, v in pairs(h) do
            ngx.say("header " .. tostring(k) .. " : " .. tostring(v))
        end
    end
    ngx.say "from backend"
end

local p = require "config.policy_fetch"
function _M.set_policy(policy)
    p.update_policy(policy, "manual")
end

function _M.set_policy_lua(policy_table)
    require("policy_helper").set_policy_lua(policy_table)
end

function _M.test_policy_cache()
    ngx.ctx.alb_ctx = {var = {uri = "/t1", trace = {}}}
    local up, policy, err = ups.get_upstream("http", "tcp", 80)
    h.assert_is_nil(err)
    h.assert_eq(up, "test-upstream-1")
    u.logs(up, policy, err)
    -- TODO FIXME wait backend sync
    ngx.sleep(3)

    local httpc = require"resty.http".new()
    do
        local res, err = httpc:request_uri("http://127.0.0.1:80/t1", {})
        u.logs(res, err)
        h.assert_eq(res.status, 200)
        h.assert_eq(res.body, "from backend\n")
    end
end

function _M.test_error_reason()
    local melf = _M
    local httpc = require"resty.http".new()
    -- backend connect fail
    do
        local res, err = httpc:request_uri("http://127.0.0.1:80/t5", {})
        u.logs(res, err)
        h.assert_eq(res.status, 502)
        h.assert_eq(res.body, "X-Error: 502\n")
    end
    -- balancer err
    do
        local res, err = httpc:request_uri("http://127.0.0.1:80/t3", {})
        u.logs(res, err)
        h.assert_eq(res.status, 500)
        h.assert_eq(res.headers["X-ALB-ERR-REASON"], "InvalidBalancer : no balancer found for test-upstream-3")

        local res, err = httpc:request_uri("http://127.0.0.1:80/t4", {})
        u.logs(res, err)
        h.assert_eq(res.status, 500)
        h.assert_eq(res.headers["X-ALB-ERR-REASON"], "InvalidBalancer : no balancer found for test-upstream-4")

        local res, err = httpc:request_uri("http://127.0.0.1:80/t1/500", {})
        u.logs(res, err)
        h.assert_eq(res.status, 500)
        h.assert_eq(res.headers["X-ALB-ERR-REASON"], "BackendError : read from backend 324")
    end
    do
        local res, err = httpc:request_uri("http://127.0.0.1:80/t1", {})
        u.logs(res, err)
        h.assert_eq(res.status, 200)
        h.assert_eq(res.headers["X-ALB-ERR-REASON"], nil)
    end
    do
        local res, err = httpc:request_uri("http://127.0.0.1:80/asdsafadfafda", {})
        u.logs(res, err)
        h.assert_eq(res.status, 404)
        h.assert_eq(res.headers["X-ALB-ERR-REASON"], "InvalidUpstream : no rule match")
    end
    do
        local res, err = httpc:request_uri("http://127.0.0.1:80/t1/404", {})
        u.logs(res, err, "should 404")
        h.assert_eq(res.status, 404)
        h.assert_eq(res.headers["X-ALB-ERR-REASON"], "BackendError : read from backend 293")
    end
    do
        local res, err = httpc:request_uri("http://127.0.0.1:28080/", {})
        u.logs(res, err)
        h.assert_eq(res.status, 404)
        h.assert_eq(res.headers["X-ALB-ERR-REASON"], "InvalidUpstream : no http_tcp_28080")
    end

    do
        local p = default_policy()
        p["http"]["tcp"]["80"] = {}
        melf.set_policy_lua(p)
        local res, err = httpc:request_uri("http://127.0.0.1:80/", {})
        h.assert_eq(res.headers["X-ALB-ERR-REASON"], "InvalidUpstream : empty policy")
        u.logs(res, err)
        melf.set_policy_lua(default_policy())
    end

    do
        local res, err = httpc:request_uri("https://127.0.0.1/", {})
        h.assert_eq(err, "handshake failed")
        u.logs(res, err)
        local p = default_policy()
        p["certificate_map"]["443"] = {
            ["key"] = "-----BEGIN PRIVATE KEY-----\n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\nYUQmiK6nOA0nP4T5x0A6LTbN6AOcxeg=\n-----END PRIVATE KEY-----",
            ["cert"] = "-----BEGIN CERTIFICATE-----\nMIIFDzCCAvegAwIBAgIURfAGhgnCVBovG1GXafPioc7ln/kwDQYJKoZIhvcNAQEL\nBQAwFzEVMBMGA1UEAwwMdGVzdC5hbGIuY29tMB4XDTIxMDkyNjAzMTkxOVoXDTMx\nMDkyNDAzMTkxOVowFzEVMBMGA1UEAwwMdGVzdC5hbGIuY29tMIICIjANBgkqhkiG\n9w0BAQEFAAOCAg8AMIICCgKCAgEAr1AlmjGNbn3pVxonTlur42btuJYxkhC+P3Vt\ntZuVNKR3LxMPlMCCJ1jnJA3y9IVzh1KuJvZQExEC1DXyOhFdQHEweLLic7NSgNEa\nBFI4gEmU56W/StjR10BBiPwqTDLRR3/rVNfEkwYPc103J7QZpBWIzLpxSPdIpnPa\nfLndROjvuXZoQsZPlQwrUQg19DKWQrCsnzNBGD1LqRajOIeY0AUmied8sMXkd0rq\nIRVe1fJbIUIvYCtMHAIS4R1S69mCdy18wdBpYfrpPamOEs3L9gtZxBeEPFLiOz7+\n6jaQI0lXJ5l3+364rOykjdYa7PRpRUPP7GkP6KBBto2x4YGKPS/UuunqL50GWzjx\naM/QCs1RyekDJCR2+4BAE79Ke2HLx6d9dj6M+Kq3vCTIsfFDR6f5kkAKnXJuNTbP\nkoIx+Gg2uwbNiPSGBKdNZ0QWRYZDG3sb+cTyFL9RifawYnncWPVKsYxeCTgc+wcm\nwMh90gzcrrkGwVAIO6gvFQFiBpOFuc0BLgfhPZ0t9BWHC3epEKuKKXwbPZbQzppp\n72wkr/pdmPngBL9gSye0O+4qL3Aoy/T/NbemkwIFXToOm+hX6Nea9QafItWN5LSg\n13fGht87+mciNZ2xTXpcJvcZ6MdtqHH8/+1iaZLleO8gCaLa55NC3ieIghiSLnnz\ntKw4mDECAwEAAaNTMFEwHQYDVR0OBBYEFJWEB7GtSJ1glNCtoJLEnRtbviL2MB8G\nA1UdIwQYMBaAFJWEB7GtSJ1glNCtoJLEnRtbviL2MA8GA1UdEwEB/wQFMAMBAf8w\nDQYJKoZIhvcNAQELBQADggIBAEkeG+Kiar98U9TB4IZpiqo2dw38Zk8fcPJK6wIT\n5104F07DCErYcBp4LXWTJX9iVsdkiiVSE/FmqQjWeX5kACUuM8HkeziQNj++EcTq\nDQtDk9zEBWGDYQH4RQZvIVQlIaieZOArSbhunIrxlGr6fX//ryLO5K4QAay/oqwb\nUXFYfQ0M7VdrsTwLRImQN5KYAJbdR/Nlm5i/fTJps5iSdgpovBAommb1XpVq/aJ5\n3tAqb64vBNigZ7T8V1cCh6oDoCO/xzoucTzF9e14LkTmtzYxBjhplgwSUH6R0cgi\ndiexT2mdBtU79iJ0K5AJFVa1UCR0OE3/FmWEkb4L01XxU5sEyYL6I0JPSXaDdjtL\nv3y2GZY2Iz27qjz/JSZXoyf28rAYE0YHI3nX1wBwDTSoPKnfSc1A/IFsXFfkGB00\nuFNiI5rRff+zBt0XCAEz2Q9aULI5Ho8kjdOqHT/ty6c9RbxnJHv3mRQy0kZsb8QM\nDHTqwEvHE7mwGtd5LD5z6SRQpCfQmoSDuNqUdxMLNIYn45+BZyAlHE6Le21fH/Rb\nCjWQ5fBl7QdBHGB9dpYu8dhdrOlN0xj1QJKJGrVkqOA4nGmD6GThBX5RDy1D0flY\npxjSbTVmKWMIaqznKYfQO88Oc1kpqZB0X6p3XT3JnCkp9wXhEidc/qVY7/nUn0/9\nU1ye\n-----END CERTIFICATE-----"
        }
        melf.set_policy_lua(p)
        local res, err = httpc:request_uri("https://127.0.0.1/", {ssl_verify = false})
        u.logs(res, err)
        h.assert_eq(res.headers["X-ALB-ERR-REASON"], "InvalidUpstream : no http_tcp_443")
        melf.set_policy_lua(default_policy())
    end
    -- timeout
    do
        local p = default_policy()
        p["http"]["tcp"]["80"] = {{rule = "1", internal_dsl = {{"STARTS_WITH", "URL", "/t1"}}, upstream = "test-upstream-1", config = {timeout = {proxy_read_timeout_ms = "3000"}}}}
        melf.set_policy_lua(p)
        local res, err = httpc:request_uri("http://127.0.0.1:80/t1/sleep?sleep=1", {})
        u.logs(res, err)
        h.assert_eq(res.status, 200)
        h.assert_eq(res.body, "from backend\n")
        -- 现在alb设置的timeout是3s，所以报错应该是alb timeout
        local res, err = httpc:request_uri("http://127.0.0.1:80/t1/sleep?sleep=5", {})
        u.logs(res, err)
        h.assert_eq(res.body, "X-Error: 504\n")
        h.assert_eq(res.status, 504)

        -- 现在是后端直接返回的504 所以报错应该是backenderror
        local res, err = httpc:request_uri("http://127.0.0.1:80/t1/timeout", {})
        u.logs(res, err)
        h.assert_eq(res.headers["X-ALB-ERR-REASON"], "BackendError : read from backend 314")
        h.assert_eq(res.status, 504)
    end
end

function _M.test_trace()
    local httpc = require"resty.http".new()
    -- with cpaas-trace it should return cpaas-trace header

    do
        local res, err = u.curl("http://127.0.0.1:80/trace1", {headers = {["cpaas-trace"] = "true"}})
        h.assert_curl_success(res, err)
        local cpaas_trace = res.headers["x-cpaas-trace"]
        h.assert_eq(type(cpaas_trace), "table")
        local trace1 = common.json_decode(cpaas_trace[1])
        u.logs(trace1)
        h.assert_eq(trace1.rule, "trace1")
        h.assert_eq(trace1.upstream, "trace1")
        local trace2 = common.json_decode(cpaas_trace[2])
        u.logs(trace2)
        h.assert_eq(trace2.rule, "trace2")
        h.assert_eq(trace2.upstream, "trace2")
    end

    do
        local res, err = u.curl("http://127.0.0.1:80/t6/detail", {headers = {["cpaas-trace"] = "true"}})
        h.assert_curl_success(res, err)
        u.logs(res)
        local cpaas_trace = res.headers["x-cpaas-trace"]
        h.assert_eq(type(cpaas_trace), "string")
        local trace = common.json_decode(cpaas_trace)
        u.logs(trace)
        h.assert_eq(trace.rule, "t6")
        h.assert_eq(trace.upstream, "test-upstream-1")
    end

    do
        local res, err = httpc:request_uri("http://127.0.0.1:80/t7/detail", {headers = {["cpaas-trace"] = "true"}})
        u.logs(res, err)
        h.assert_eq(res.status, 200)
        local trace = common.json_decode(res.headers["x-cpaas-trace"])
        h.assert_eq(trace.rule, "t7")
        h.assert_eq(trace.upstream, "test-upstream-1")
        u.logs(trace)
    end
    do
        local res, err = httpc:request_uri("http://127.0.0.1:80/t1/detail", {headers = {["cpaas-trace"] = "true"}})
        u.logs(res, err)
        h.assert_eq(res.status, 200)
        local trace = common.json_decode(res.headers["x-cpaas-trace"])
        h.assert_eq(trace.rule, "1")
        h.assert_eq(trace.upstream, "test-upstream-1")
        u.logs(trace)
    end
    -- second
    do
        local res, err = httpc:request_uri("http://127.0.0.1:80/t2", {headers = {["cpaas-trace"] = "true"}})
        u.logs(res, err)
        h.assert_eq(res.status, 200)
        h.assert_eq(res.body, "from backend\n")
        local trace = common.json_decode(res.headers["x-cpaas-trace"])
        h.assert_eq(trace.rule, "2")
        h.assert_eq(trace.upstream, "test-upstream-1")
        u.logs(trace)
    end
end

function _M.test()
    local s = _M
    u.logs "in test"

    s.set_policy_lua(default_policy())
    s.test_error_reason()
    s.set_policy_lua(default_policy())
    s.test_policy_cache()

    s.set_policy_lua(default_policy())
    s.test_trace()

    s.set_policy_lua(default_policy())
    s.test_metrics()
end

local function get_metrics()
    local res, err = u.httpc():request_uri("http://127.0.0.1:1936/metrics", {})
    h.assert_is_nil(err)
    return res.body
end

local function clear_metrics()
    local res, err = u.httpc():request_uri("http://127.0.0.1:1936/clear", {})
    h.assert_is_nil(err)
    return res.body
end

function _M.test_metrics()
    clear_metrics()
    local httpc = require"resty.http".new()
    do
        local res, err = httpc:request_uri("http://127.0.0.1:80/asdsafadfafda", {})
        u.logs(res, err)
        h.assert_eq(res.status, 404)
        h.assert_eq(res.headers["X-ALB-ERR-REASON"], "InvalidUpstream : no rule match")
        h.assert_contains(get_metrics(), [[alb_error{port="80"} 1]])
    end
    do
        local res, err = httpc:request_uri("http://127.0.0.1:80/t1/404", {})
        u.logs(res, err)
        h.assert_eq(res.status, 404)
        h.assert_eq(res.headers["X-ALB-ERR-REASON"], "BackendError : read from backend 293")
        h.assert_contains(get_metrics(), [[alb_error{port="80"} 1]])
    end
    do
        local res, err = httpc:request_uri("http://127.0.0.1:80/asdsafadfafda", {})
        u.logs(res, err)
        h.assert_eq(res.status, 404)
        h.assert_eq(res.headers["X-ALB-ERR-REASON"], "InvalidUpstream : no rule match")
        h.assert_contains(get_metrics(), [[alb_error{port="80"} 2]])
    end
end

return _M
