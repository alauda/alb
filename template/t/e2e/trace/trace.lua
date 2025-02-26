-- format:on
local _M = {}
local u = require "util"
local h = require "test-helper"
local common = require "utils.common"
local ups = require "match_engine.upstream"
local ph = require "policy_helper"

local default_443_cert =
"-----BEGIN CERTIFICATE-----\nMIIFFTCCAv2gAwIBAgIUNcaMWCswms56XCvj8nxC/5AKxtUwDQYJKoZIhvcNAQEL\nBQAwGjEYMBYGA1UEAwwPNDQzLmRlZmF1bHQuY29tMB4XDTIyMDUxOTA5MjEzMVoX\nDTMyMDUxNjA5MjEzMVowGjEYMBYGA1UEAwwPNDQzLmRlZmF1bHQuY29tMIICIjAN\nBgkqhkiG9w0BAQEFAAOCAg8AMIICCgKCAgEAvMqeEYs9K/DrbZ3FXj7yZaVRexub\na4OF0S/jg2qXrTK8FwPQ1MJiVxPL2jNeE7PT1fCNujb3+fZ/way99FJ0KpmbqEeP\nGt8490oqHZl7LuiEklrSiNp5qOJERsxgNrtq5RILIC1wH9eu0dNilwnCldzEXqFJ\n4+vZqPQfNxk//0vSOxsapl/nEPze6aMy+sUnyFJoq3ti/O02sV/p5sOQX3NcPoXU\n23PTr1xMDVQ7IpuR4GkxbmIVdAMuGWA2udYN0H3ou1VVy+je3RVF7xD2V/lMI3RL\nzLinfWxyNBUOoswylWRjdgwfrz5EkGuN58uT+o28Lx0APw06gwZ0eXb0cKdaYM0X\n04p4d3r//KLgm5WZpvDrjCC3aP02Yk1rITAu9owx+fNjIuEuPJtfin6r9Cjed7xL\n9CgdlFONDkNPxMz52qnf9Jbuf4HPTa/jDw7ICG8FAR8RwljJ7ohFCmullfXtumpX\nbT8+4DK1+H1fqkLV4lCWtQwn8ULqqCDQJZszco5KcnNnenqKgNPLVe7t6/ZxDZ8j\nMYAyGIR+DMDp0tLjfHD26IjEzF/n3E0pZiTXFaRirjKcFd523qEWvZeZc0nSykhH\nBUYXgxh2Nqi3Cv9VxA6sHVto5GvQBWq0kl6Qo9IGof51+4HCm++8/bCpf2Gcv/kI\ny39JIHMSGCa4ztcCAwEAAaNTMFEwHQYDVR0OBBYEFDawzxBvztJOekhp/DU9GKo+\nsnc9MB8GA1UdIwQYMBaAFDawzxBvztJOekhp/DU9GKo+snc9MA8GA1UdEwEB/wQF\nMAMBAf8wDQYJKoZIhvcNAQELBQADggIBACd2Z9XyESvQ4MfYMUID2DCmuVGBDhyo\n8cN88nuy+plrcYSpsthp55C+dhhfJRocES0NkpIVojpiQiPQdAyLFKb1M1Mcd9bg\n+qtYrOH2lS0Uem2s366D8LLJSOzWv/f75wUHe3eyivzW73zcM3znr5TrAFrCkUBF\npkK90G1VEznpD+VDvXYfcXklTZ7lMVZJ1ck2MDYPkh3nGtCyY6z+r41vJo/OcW8A\ncxicgsKXjEiXOH42B8ugad5gK27gA/FKwtTNPPU4K0UeDCAJaY+L7USjbrUgeQ17\nmjCOrY53OjyjjD4YjsE9EqsU/Hc9lqIUdCktZEDrLKfjGT1raaqDlSzEYYcs/oai\n0Ka3MXao2czYEJz6YZIOtp7FatRUBajCZ3NJeTgPFMZn10g7CktJR5QJDvvqbUBs\nHCddmahNPdgQwjxGVfoAI5SDH2QnIlj3bLivU+4oqR7hO7Nmhx9BtNRdHhM+M+wp\nsLvVETvtZdHC3RX4rX4pAl/r7pjhC7n0tbn3XyK96yZ4Yu/E+d/Cqhs0+rssqLzH\nDtMZCMOsaZi1AUEtc2cmZweOXEHeEoyPn3nJeVLfW2+dThlK/i9RaZbPThTS/GdK\nCU530BEDG+y/I5p6dndYySm2+LJiC0Xso1S1gLa7NccV8Y1E9Y8026J3lpvMilhP\nBwA4jE77yBPI\n-----END CERTIFICATE-----";
local default_443_key =
"-----BEGIN RSA PRIVATE KEY-----\n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\n-----END RSA PRIVATE KEY-----";

-- LuaFormatter off
local function default_policy()
    return {
        certificate_map = {
            ["1936"] = { cert = default_443_cert, key = default_443_key },
        },
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
        ---@cast t -nil

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

function _M.set_policy_lua(policy_table)
    ph.set_policy_lua(policy_table)
end

function _M.test_policy_cache()
    ngx.ctx.alb_ctx = { var = { uri = "/t1", trace = {} } }
    local up, policy, err = ups.get_upstream("http", "tcp", 80)
    h.assert_is_nil(err)
    h.assert_eq(up, "test-upstream-1")
    u.logs(up, policy, err)

    local httpc = require "resty.http".new()
    do
        local res, err = httpc:request_uri("http://127.0.0.1:80/t1", {})
        u.logs(res, err)
        h.assert_eq(res.status, 200)
        h.assert_eq(res.body, "from backend\n")
    end
end

function _M.test_error_reason()
    local melf = _M
    local httpc = require "resty.http".new()
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
        h.assert_eq(res.headers["X-ALB-ERR-REASON"], "BackendError : read 324 byte data from backend")
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
        h.assert_eq(res.headers["X-ALB-ERR-REASON"], "BackendError : read 293 byte data from backend")
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
            ["key"] =
            "-----BEGIN PRIVATE KEY-----\n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\nYUQmiK6nOA0nP4T5x0A6LTbN6AOcxeg=\n-----END PRIVATE KEY-----",
            ["cert"] =
            "-----BEGIN CERTIFICATE-----\nMIIFDzCCAvegAwIBAgIURfAGhgnCVBovG1GXafPioc7ln/kwDQYJKoZIhvcNAQEL\nBQAwFzEVMBMGA1UEAwwMdGVzdC5hbGIuY29tMB4XDTIxMDkyNjAzMTkxOVoXDTMx\nMDkyNDAzMTkxOVowFzEVMBMGA1UEAwwMdGVzdC5hbGIuY29tMIICIjANBgkqhkiG\n9w0BAQEFAAOCAg8AMIICCgKCAgEAr1AlmjGNbn3pVxonTlur42btuJYxkhC+P3Vt\ntZuVNKR3LxMPlMCCJ1jnJA3y9IVzh1KuJvZQExEC1DXyOhFdQHEweLLic7NSgNEa\nBFI4gEmU56W/StjR10BBiPwqTDLRR3/rVNfEkwYPc103J7QZpBWIzLpxSPdIpnPa\nfLndROjvuXZoQsZPlQwrUQg19DKWQrCsnzNBGD1LqRajOIeY0AUmied8sMXkd0rq\nIRVe1fJbIUIvYCtMHAIS4R1S69mCdy18wdBpYfrpPamOEs3L9gtZxBeEPFLiOz7+\n6jaQI0lXJ5l3+364rOykjdYa7PRpRUPP7GkP6KBBto2x4YGKPS/UuunqL50GWzjx\naM/QCs1RyekDJCR2+4BAE79Ke2HLx6d9dj6M+Kq3vCTIsfFDR6f5kkAKnXJuNTbP\nkoIx+Gg2uwbNiPSGBKdNZ0QWRYZDG3sb+cTyFL9RifawYnncWPVKsYxeCTgc+wcm\nwMh90gzcrrkGwVAIO6gvFQFiBpOFuc0BLgfhPZ0t9BWHC3epEKuKKXwbPZbQzppp\n72wkr/pdmPngBL9gSye0O+4qL3Aoy/T/NbemkwIFXToOm+hX6Nea9QafItWN5LSg\n13fGht87+mciNZ2xTXpcJvcZ6MdtqHH8/+1iaZLleO8gCaLa55NC3ieIghiSLnnz\ntKw4mDECAwEAAaNTMFEwHQYDVR0OBBYEFJWEB7GtSJ1glNCtoJLEnRtbviL2MB8G\nA1UdIwQYMBaAFJWEB7GtSJ1glNCtoJLEnRtbviL2MA8GA1UdEwEB/wQFMAMBAf8w\nDQYJKoZIhvcNAQELBQADggIBAEkeG+Kiar98U9TB4IZpiqo2dw38Zk8fcPJK6wIT\n5104F07DCErYcBp4LXWTJX9iVsdkiiVSE/FmqQjWeX5kACUuM8HkeziQNj++EcTq\nDQtDk9zEBWGDYQH4RQZvIVQlIaieZOArSbhunIrxlGr6fX//ryLO5K4QAay/oqwb\nUXFYfQ0M7VdrsTwLRImQN5KYAJbdR/Nlm5i/fTJps5iSdgpovBAommb1XpVq/aJ5\n3tAqb64vBNigZ7T8V1cCh6oDoCO/xzoucTzF9e14LkTmtzYxBjhplgwSUH6R0cgi\ndiexT2mdBtU79iJ0K5AJFVa1UCR0OE3/FmWEkb4L01XxU5sEyYL6I0JPSXaDdjtL\nv3y2GZY2Iz27qjz/JSZXoyf28rAYE0YHI3nX1wBwDTSoPKnfSc1A/IFsXFfkGB00\nuFNiI5rRff+zBt0XCAEz2Q9aULI5Ho8kjdOqHT/ty6c9RbxnJHv3mRQy0kZsb8QM\nDHTqwEvHE7mwGtd5LD5z6SRQpCfQmoSDuNqUdxMLNIYn45+BZyAlHE6Le21fH/Rb\nCjWQ5fBl7QdBHGB9dpYu8dhdrOlN0xj1QJKJGrVkqOA4nGmD6GThBX5RDy1D0flY\npxjSbTVmKWMIaqznKYfQO88Oc1kpqZB0X6p3XT3JnCkp9wXhEidc/qVY7/nUn0/9\nU1ye\n-----END CERTIFICATE-----"
        }
        melf.set_policy_lua(p)
        local res, err = httpc:request_uri("https://127.0.0.1/", { ssl_verify = false })
        u.logs(res, err)
        h.assert_eq(res.headers["X-ALB-ERR-REASON"], "InvalidUpstream : no http_tcp_443")
        melf.set_policy_lua(default_policy())
    end
    -- timeout
    do
        local p = default_policy()
        p["http"]["tcp"]["80"] = { { plugins = { "timeout" }, rule = "1", internal_dsl = { { "STARTS_WITH", "URL", "/t1" } }, upstream = "test-upstream-1", config = { timeout = { proxy_read_timeout_ms = 3000 } } } }
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
        h.assert_eq(res.headers["X-ALB-ERR-REASON"], "BackendError : read 314 byte data from backend")
        h.assert_eq(res.status, 504)
    end
end

function _M.test_trace()
    local httpc = require "resty.http".new()
    -- with cpaas-trace it should return cpaas-trace header

    do
        local res, err = u.curl("http://127.0.0.1:80/trace1", { headers = { ["cpaas-trace"] = "true" } })
        h.assert_curl_success(res, err)
        local cpaas_trace = res.headers["x-cpaas-trace"]
        h.assert_eq(type(cpaas_trace), "table")
        local trace1 = common.json_decode(cpaas_trace[1])
        u.logs(trace1)
        ---@cast trace1 -nil
        h.assert_eq(trace1.rule, "trace1")
        h.assert_eq(trace1.upstream, "trace1")
        local trace2 = common.json_decode(cpaas_trace[2])
        ---@cast trace2 -nil
        u.logs(trace2)
        h.assert_eq(trace2.rule, "trace2")
        h.assert_eq(trace2.upstream, "trace2")
    end

    do
        local res, err = u.curl("http://127.0.0.1:80/t6/detail", { headers = { ["cpaas-trace"] = "true" } })
        h.assert_curl_success(res, err)
        u.logs(res)
        local cpaas_trace = res.headers["x-cpaas-trace"]
        h.assert_eq(type(cpaas_trace), "string")
        local trace = common.json_decode(cpaas_trace) or {}
        u.logs(trace)
        h.assert_eq(trace.rule, "t6")
        h.assert_eq(trace.upstream, "test-upstream-1")
    end

    do
        local res, err = httpc:request_uri("http://127.0.0.1:80/t7/detail", { headers = { ["cpaas-trace"] = "true" } })
        u.logs(res, err)
        h.assert_eq(res.status, 200)
        local trace = common.json_decode(res.headers["x-cpaas-trace"]) or {}
        h.assert_eq(trace.rule, "t7")
        h.assert_eq(trace.upstream, "test-upstream-1")
        u.logs(trace)
    end
    do
        local res, err = httpc:request_uri("http://127.0.0.1:80/t1/detail", { headers = { ["cpaas-trace"] = "true" } })
        u.logs(res, err)
        h.assert_eq(res.status, 200)
        local trace = common.json_decode(res.headers["x-cpaas-trace"]) or {}
        h.assert_eq(trace.rule, "1")
        h.assert_eq(trace.upstream, "test-upstream-1")
        u.logs(trace)
    end
    -- second
    do
        local res, err = httpc:request_uri("http://127.0.0.1:80/t2", { headers = { ["cpaas-trace"] = "true" } })
        u.logs(res, err)
        h.assert_eq(res.status, 200)
        h.assert_eq(res.body, "from backend\n")
        local trace = common.json_decode(res.headers["x-cpaas-trace"]) or {}
        h.assert_eq(trace.rule, "2")
        h.assert_eq(trace.upstream, "test-upstream-1")
        u.logs(trace)
    end
end

local function get_metrics()
    local res, err = u.httpc():request_uri("https://127.0.0.1:1936/metrics", { ssl_verify = false })
    h.assert_is_nil(err)
    return res.body
end

local function clear_metrics()
    local res, err = u.httpc():request_uri("https://127.0.0.1:1936/clear", { ssl_verify = false })
    h.assert_is_nil(err)
    return res.body
end

function _M.test_metrics()
    clear_metrics()
    local httpc = require "resty.http".new()
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
        h.assert_eq(res.headers["X-ALB-ERR-REASON"], "BackendError : read 293 byte data from backend")
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

function _M.test()
    local s = _M
    u.logs "in test"
    u.logs "test error reason"
    s.set_policy_lua(default_policy())
    u.logs "after test error reason"
    s.test_error_reason()
    u.logs "test policy cache. set policy"
    s.set_policy_lua(default_policy())
    u.logs "test policy cache. after set policy"
    s.test_policy_cache()

    s.set_policy_lua(default_policy())
    s.test_trace()
    u.logs "test metrics"
    s.set_policy_lua(default_policy())
    s.test_metrics()
end

return _M
