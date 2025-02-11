-- format:on
local _M = {}
local h = require "test-helper"
local u = require "util"
local ph = require("policy_helper")

function _M.as_backend()
    ngx.log(ngx.ERR, "=======  url  ========")
    ngx.log(ngx.ERR, ngx.var.scheme .. "://" .. ngx.var.host .. ngx.var.request_uri)
    ngx.log(ngx.ERR, ngx.var.uri)
    ngx.log(ngx.ERR, "=======  h  ========")
    local h = ngx.req.get_headers()
    for k, v in pairs(h) do
        ngx.log(ngx.ERR, tostring(k) .. " " .. tostring(v))
    end
    ngx.log(ngx.ERR, "=======  h  ========")
    ngx.req.read_body()
    local body = ngx.req.get_body_data()
    ngx.log(ngx.ERR, "=======  body  ========")
    ngx.log(ngx.ERR, body)
    ngx.log(ngx.ERR, "=======  body  ========")
    if ngx.var.uri == "/p3" then
        local ret = ngx.req.get_uri_args()["ret"]
        ngx.header["X-RET"] = ret
        ngx.log(ngx.ERR, "param_ret: " .. ret)
        ngx.say("ok")
        return
    end
    if ngx.var.uri == "/p4" then
        local ret = ngx.req.get_uri_args()["ret"]
        ngx.say(ret)
        return
    end
    ngx.say("ok")
end

function _M.test()
    ph.set_policy_lua({
        http = {
            tcp = {
                ["80"] = {
                    {
                        rule = "p1",
                        internal_dsl = { { "STARTS_WITH", "URL", "/p1" } },
                        to_location = "modsecurity_p1",
                        upstream = "test-upstream-1"
                    },
                    {
                        rule = "redirect_p1",
                        internal_dsl = { { "STARTS_WITH", "URL", "/redirect_p1" } },
                        to_location = "modsecurity_redirect_p1",
                        upstream = "test-upstream-1"
                    },
                    {
                        rule = "p2",
                        internal_dsl = { { "STARTS_WITH", "URL", "/p2" } },
                        to_location = "modsecurity_p2",
                        upstream = "test-upstream-1"
                    },
                    {
                        rule = "p3",
                        internal_dsl = { { "STARTS_WITH", "URL", "/p3" } },
                        to_location = "modsecurity_p3",
                        upstream = "test-upstream-1"
                    },
                    {
                        rule = "p4",
                        internal_dsl = { { "STARTS_WITH", "URL", "/p4" } },
                        to_location = "modsecurity_p4",
                        upstream = "test-upstream-1"
                    },
                    {
                        rule = "p-rule",
                        internal_dsl = { { "STARTS_WITH", "URL", "/p-rule" } },
                        to_location = "modsecurity_rule",
                        upstream = "test-upstream-1"
                    },
                    {
                        rule = "1",
                        internal_dsl = { { "STARTS_WITH", "URL", "/t2" } },
                        upstream = "test-upstream-1"
                    },
                }
            }
        },
        backend_group = {
            { name = "test-upstream-1", mode = "http", backends = { { address = "127.0.0.1", port = 1880, weight = 100 } } }
        }
    })

    -- hit p1 rule
    local res, err = h.just_curl("http://127.0.0.1:80/p1?testparam=test")
    u.logs("curl err", err)
    h.assert_eq(err, nil)
    u.logs("curl res", h.curl_res_to_string(res))
    h.assert_eq(res.status, 403)

    local res, err = h.just_curl("http://127.0.0.1:80/redirect_p1?testparam=redirect")
    h.assert_eq(err, nil)
    u.logs(h.curl_res_to_string(res))
    h.assert_eq(res.status, 302)
    h.assert_eq(res.headers["Location"], "http://a.com")


    -- not hit p1 rule
    local res, err = h.just_curl("http://127.0.0.1:80/p1?testparam=ok", { body = "xx" })
    u.logs(h.curl_res_to_string(res))
    h.assert_curl_success(res, err, "ok")

    -- hit p2 rule
    local res, err = h.just_curl("http://127.0.0.1:80/p2?x=x", {
        body = [[ {"a":"b"}]],
        headers = {
            ["Content-Type"] = "application/json"
        }
    })
    h.assert_eq(err, nil)
    u.logs(h.curl_res_to_string(res))
    h.assert_eq(res.status, 403)
    -- not hit p2 rule
    local res, err = h.just_curl("http://127.0.0.1:80/p2?x=x", {
        body = [[ {"a":"c"}]],
        headers = {
            ["Content-Type"] = "application/json"
        }
    })
    h.assert_curl_success(res, err, "ok")



    -- hit p3 rule
    local res, err = h.just_curl("http://127.0.0.1:80/p3?ret=b")
    h.assert_eq(err, nil)
    u.logs(h.curl_res_to_string(res))
    h.assert_eq(res.status, 403)
    -- not hit p3 rule
    local res, err = h.just_curl("http://127.0.0.1:80/p3?ret=c")
    h.assert_eq(err, nil)
    u.logs(h.curl_res_to_string(res))
    h.assert_curl_success(res, err, "ok")

    -- hit p4 rule
    -- p4 rule just not worked due https://github.com/owasp-modsecurity/ModSecurity-nginx/issues/61

    -- t2 not enable waf
    local res, err = h.just_curl("http://127.0.0.1:80/t2?testparam=test")
    h.assert_curl_success(res, err, "ok")

    -- hit pre defined rule
    local res, err = h.just_curl("http://127.0.0.1:80/p-rule/asd/../;")
    u.logs(res, err)
    h.assert_eq(res.status, 403)

    -- h.assert_eq(err, nil)
    -- u.logs(h.curl_res_to_string(res))
    -- h.assert_curl_success(res, err, "ok")
end

return _M
