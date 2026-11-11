local _m = {}
local u = require("util")
local ph = require("policy_helper")
local h = require "test-helper"
local c = require "utils.common"

function _m.as_backend()
    u.logs("as backend", ngx.req.get_headers())
    ngx.say(c.json_encode(ngx.req.get_headers()))
end

function _m.eval_case(cfg_case)
    local policy = {
        http = {
            tcp = {
                [80] = {
                    {
                        rule = cfg_case.rule,
                        internal_dsl = { { "STARTS_WITH", "URL", "/" .. cfg_case.rule } },
                        config = { refs = {} },
                        upstream = "u1",
                        backend_protocol = "http",
                        subsystem = "http",
                    },
                }
            }
        },
        config = {},
        backend_group = { { name = "u1", mode = "http", backends = { { address = "127.0.0.1", port = 1880, weight = 100 } } } }
    }
    if cfg_case.use_forwarded_headers ~= nil then
        policy.http.tcp[80][1].config.refs = { proxy_header = "proxy_header-1" }
        policy.config["proxy_header-1"] = {
            type = "proxy_header",
            proxy_header = {
                use_forwarded_headers = cfg_case.use_forwarded_headers,
            }
        }
    end
    if cfg_case.rewrite_request ~= nil then
        policy.http.tcp[80][1].config.rewrite_request = cfg_case.rewrite_request
    end
    ph.set_policy_lua(policy)
    local res, err = h.just_curl("http://127.0.0.1:80/" .. cfg_case.rule, {
        headers = cfg_case.request_headers,
    })
    h.assert_is_nil(err)
    local upstream_request_header = c.json_decode(res.body)
    upstream_request_header["user-agent"] = nil
    cfg_case.check(upstream_request_header)
end

function _m.test()
    -- use-forwarded-headers=true
    _m.eval_case({
        rule = "r1",
        use_forwarded_headers = true,
        request_headers = {
            ["x-forwarded-port"] = "8888",
            ["x-forwarded-proto"] = "not-http",
            ["x-forwarded-123"] = "223",
        },
        rewrite_request = {
        },
        check = function (headers)
            h.assert_table_equals(headers, {
                ["host"] = "127.0.0.1",
                ["x-forwarded-port"] = "8888",
                ["x-forwarded-proto"] = "not-http",
                ["x-forwarded-123"] = "223",
                ["x-original-uri"] = "/r1",
                ["x-original-url"] = "http://127.0.0.1/r1",
                ["x-original-method"] = "GET",
                ["x-real-ip"] = "127.0.0.1",
                ["x-scheme"] = "http"
            })
        end
    })

    -- 当没有配置任何参数时,和之前的alb的逻辑一致
    _m.eval_case({
        rule = "r1",
        use_forwarded_headers = nil,
        request_headers = {
        },
        rewrite_request = {
        },
        check = function (headers)
            h.assert_table_equals(headers, {
                ["host"] = "127.0.0.1",
                ["x-forwarded-for"] = "127.0.0.1",
                ["x-forwarded-host"] = "127.0.0.1",
                ["x-forwarded-port"] = "80",
                ["x-forwarded-proto"] = "http",
                ["x-original-method"] = "GET",
                ["x-original-uri"] = "/r1",
                ["x-original-url"] = "http://127.0.0.1/r1",
                ["x-real-ip"] = "127.0.0.1",
                ["x-scheme"] = "http"
            })
        end
    })

    -- -- 增加x-forward-for
    _m.eval_case({
        rule = "r1",
        use_forwarded_headers = nil,
        request_headers = {
            ["x-forwarded-for"] = "10.0.0.1",
        },
        rewrite_request = {
        },
        check = function (headers)
            h.assert_table_equals(headers, {
                ["host"] = "127.0.0.1",
                ["x-forwarded-for"] = "10.0.0.1, 127.0.0.1",
                ["x-forwarded-host"] = "127.0.0.1",
                ["x-forwarded-port"] = "80",
                ["x-forwarded-proto"] = "http",
                ["x-original-method"] = "GET",
                ["x-original-uri"] = "/r1",
                ["x-original-url"] = "http://127.0.0.1/r1",
                ["x-real-ip"] = "127.0.0.1",
                ["x-scheme"] = "http"
            })
        end
    })

    -- -- 不会使用client request中的x-forwarded-xx的header
    _m.eval_case({
        rule = "r1",
        use_forwarded_headers = nil,
        request_headers = {
            ["x-forwarded-port"] = "8888",
            ["x-forwarded-proto"] = "not-http",
            ["x-forwarded-123"] = "223",
        },
        rewrite_request = {
        },
        check = function (headers)
            h.assert_table_equals(headers, {
                ["host"] = "127.0.0.1",
                ["x-forwarded-123"] = "223",
                ["x-forwarded-for"] = "127.0.0.1",
                ["x-forwarded-host"] = "127.0.0.1",
                ["x-forwarded-port"] = "80",
                ["x-forwarded-proto"] = "http",
                ["x-original-method"] = "GET",
                ["x-original-uri"] = "/r1",
                ["x-original-url"] = "http://127.0.0.1/r1",
                ["x-real-ip"] = "127.0.0.1",
                ["x-scheme"] = "http"
            })
        end
    })
    -- rewrite-request could delete any header
    _m.eval_case({
        rule = "r1",
        use_forwarded_headers = nil,
        request_headers = {
            ["x-forwarded-port"] = "8888",
            ["x-forwarded-proto"] = "not-http",
            ["x-forwarded-123"] = "223",
        },
        rewrite_request = {
            headers_remove = {
                "x-original-uri",
                "x-forwarded-host",
                "x-forwarded-123",
            }
        },
        check = function (headers)
            h.assert_table_equals(headers, {
                ["host"] = "127.0.0.1",
                ["x-forwarded-for"] = "127.0.0.1",
                ["x-forwarded-port"] = "80",
                ["x-forwarded-proto"] = "http",
                ["x-original-method"] = "GET",
                ["x-original-url"] = "http://127.0.0.1/r1",
                ["x-real-ip"] = "127.0.0.1",
                ["x-scheme"] = "http"
            })
        end
    })
    -- rewrite-request could set x-real-ip to first_forward_or_remote_addr
    _m.eval_case({
        rule = "r1",
        use_forwarded_headers = nil,
        request_headers = {
            ["x-forwarded-for"] = "123.0.0.1,127.0.0.2",
        },
        rewrite_request = {
            headers_var = {
                ["x-real-ip"] = "first_forward_or_remote_addr",
            }
        },
        check = function (headers)
            h.assert_eq(headers["x-real-ip"], "123.0.0.1")
        end
    })
    _m.eval_case({
        rule = "r1",
        use_forwarded_headers = nil,
        request_headers = {
            ["x-forwarded-for"] = "123.0.0.1",
        },
        rewrite_request = {
            headers_var = {
                ["x-real-ip"] = "first_forward_or_remote_addr",
            }
        },
        check = function (headers)
            h.assert_eq(headers["x-real-ip"], "123.0.0.1")
        end
    })
    _m.eval_case({
        rule = "r1",
        use_forwarded_headers = nil,
        request_headers = {
        },
        rewrite_request = {
            headers_var = {
                ["x-real-ip"] = "first_forward_or_remote_addr",
            }
        },
        check = function (headers)
            h.assert_eq(headers["x-real-ip"], "127.0.0.1")
        end
    })
end

return _m
