-- format:on style:emmy

local _m = {}
local h = require "test-helper"
local u = require "util"
local ph = require("policy_helper")
local common = require("utils.common")
local clone = require "table.clone"

local G = {}

function _m.as_backend()
    ngx.log(ngx.INFO, "as_backend  enter", ngx.var.uri)

    if ngx.var.uri == "/echo" then
        ngx.say("hello")
        ngx.exit(200)
        return
    end

    local id = ngx.var.arg_id
    if not id then
        ngx.status = 400
        ngx.say("id is required")
        ngx.exit(400)
        return
    end
    if not G[id] then
        G[id] = {}
    end
    table.insert(G[id], {
        uri     = ngx.var.uri,
        method  = ngx.req.get_method(),
        args    = ngx.req.get_uri_args(),
        headers = ngx.req.get_headers(),
        body    = ngx.req.get_body_data(),
    })
    -- u.logs("as_backend", ngx.var.uri, G[id])
    local code = tonumber(ngx.var.arg_code or "200") or 999
    ngx.status = code
    if ngx.var.uri == "/auth" then
        if ngx.var.arg_ret_header then
            for k, v in string.gmatch(ngx.var.arg_ret_header, "(%w+)@(%w+)") do
                ngx.log(ngx.INFO, "as_backend  set header ", k, v)
                ngx.header[k] = v
            end
            ngx.log(ngx.INFO, "as_backend  set header ", "Set-Cookie", "cc=xx")
            ngx.header["Set-Cookie"] = "cc=xx"
        end
    end
    if ngx.var.uri == "/cookie" then
        ngx.log(ngx.INFO, "set cookie " .. tostring(ngx.var.arg_auth))
        if ngx.var.arg_auth == "simple" then
            ngx.header["Set-Cookie"] = "ca=cb"
        else
            ngx.header["Set-Cookie"] = {
                "id=a3fWa; Expires=Thu, 21 Oct 2021 07:28:00 GMT; Secure; HttpOnly; Domain=mozilla.org",
                "xid=xa3fWa; Expires=Thu, 21 Oct 2021 07:28:00 GMT; Secure; HttpOnly; Domain=mozilla.org"
            }
        end
    end
    if ngx.var.uri == "/id" or ngx.var.arg_ret_data then
        ngx.say(common.json_encode(G[id]))
    end
    ngx.log(ngx.INFO, "as_backend  exit with ", code)

    if ngx.var.uri == "/" then
        if ngx.var.arg_app_cookie == "simple" then
            ngx.log(ngx.INFO, "as app  cookie ", code)
            ngx.header["Set-Cookie"] = "app=app"
        end
    end
    if code ~= 200 then
        ngx.say(ngx.var.uri .. " fail")
    end

    ngx.exit(code)
end

function _m.test()
    -- _m.test_common()
    -- _m.test_cookie()
    _m.test_basic()
end

function _m.test_cookie()
    -- cookie set by forward authentication server
    -- user retains cookie by default
    -- user does not retain cookie if upstream returns error status code
    -- user with annotated ingress retains cookie if upstream returns error status code
    ---@type NgxPolicy
    local policy = {
        certificate_map = {},
        stream = {},
        http = {
            tcp = {
                [80] = {
                    { plugins = { "auth" }, rule = "1", internal_dsl = { { "STARTS_WITH", "URL", "/" } }, config = { refs = { auth = "auth-1" } }, upstream = "u1" },
                }
            }
        },
        config = {
            ["auth-1"] = {
                type = "auth",
                auth = {
                    forward_auth = {
                        url = { "http://", "$host", ":1880", "/cookie?id=", "$arg_id", "&auth=", "$arg_auth" },
                        always_set_cookie = false,
                        method = "GET",
                        auth_headers = {},
                        auth_request_redirect = {},
                        upstream_headers = {},
                        signin_url = {},
                    },
                }
            },
        },
        backend_group = { { name = "u1", mode = "http", backends = { { address = "127.0.0.1", port = 1880, weight = 100 } }, } }
    }
    ph.set_policy_lua(policy)
    u.logs("hello")
    do
        u.logs("success should has cookie")
        local res, err = h.just_curl("http://127.0.0.1/?id=c1&auth=simple&app_cookie=simple")
        h.assert_is_nil(err)
        u.logs("check cookie ", res.headers["Set-Cookie"])
        u.logs("res ", res)
        h.assert_eq(res.headers["Set-Cookie"], { "app=app", "ca=cb" })
        u.logs("res headers ", res.headers)
    end
    do
        u.logs("fail should not set cookie")
        local res, err = u.curl("http://127.0.0.1/?id=c2&auth=simple&code=500")
        h.assert_is_nil(err)
        u.logs("check cookie ", res.headers["Set-Cookie"])
        u.logs("res", res)
        h.assert_eq(res.headers["Set-Cookie"], nil)
    end

    do
        u.logs("always_set_cookie + fail should set cookie")
        local p = clone(policy)
        p["config"]["auth-1"]["auth"]["forward_auth"]["always_set_cookie"] = true
        ph.set_policy_lua(policy)
        local res, err = h.just_curl("http://127.0.0.1/?id=c2&auth=simple&code=500")
        h.assert_is_nil(err)
        u.logs("check cookie ", res.headers["Set-Cookie"])
        u.logs("res headers ", res)
        h.assert_eq(res.headers["Set-Cookie"], "ca=cb")
    end
end

function _m.test_common()
    local cases = {
        {
            case = [[auth should ok ]],
            ingress = "",
            do_test = function ()
                ---@type NgxPolicy
                local policy = {
                    certificate_map = {},
                    stream = {},
                    http = {
                        tcp = {
                            [80] = {
                                { plugins = { "auth" }, rule = "1", internal_dsl = { { "STARTS_WITH", "URL", "/t1" } },   config = { refs = { auth = "auth-1" } },        upstream = "u1" },
                                { plugins = { "auth" }, rule = "2", internal_dsl = { { "STARTS_WITH", "URL", "/t2" } },   config = { refs = { auth = "auth-redirect" } }, upstream = "u1" },
                                { plugins = { "auth" }, rule = "3", internal_dsl = { { "STARTS_WITH", "URL", "/t3" } },   config = { refs = { auth = "auth-fail" } },     upstream = "u1" },
                                { plugins = { "auth" }, rule = "4", internal_dsl = { { "STARTS_WITH", "URL", "/t4" } },   config = { refs = { auth = "auth-https" } },    upstream = "u1" },
                                { plugins = { "auth" }, rule = "5", internal_dsl = { { "STARTS_WITH", "URL", "/auth" } }, upstream = "u1" }
                            }
                        }
                    },
                    config = {
                        ["auth-1"] = {
                            type = "auth",
                            auth = {
                                forward_auth = {
                                    always_set_cookie = false,
                                    invalid_auth_req_cm_ref = false,
                                    url = { "http://", "$host", ":1880", "/auth?id=", "$arg_id", "&ret_header=aa@bb" },
                                    method = "POST",
                                    auth_headers = { ["My-Custom-Header"] = { "42" } },
                                    auth_request_redirect = { "http://", "$host", "/", "$arg_id" },
                                    upstream_headers = { "aa" },
                                    signin_url = { "http://", "$host", "/signin" }
                                },
                            }
                        },
                        ["auth-https"] = {
                            type = "auth",
                            auth = {
                                forward_auth = {
                                    invalid_auth_req_cm_ref = false,
                                    always_set_cookie = false,
                                    url = { "https://", "$host", "/auth?id=", "$arg_id", "&ret_header=aa@bb" },
                                    method = "POST",
                                    auth_headers = { ["My-Custom-Header"] = { "42" } },
                                    auth_request_redirect = { "http://", "$host", "/", "$arg_id" },
                                    upstream_headers = { "aa" },
                                    signin_url = {}
                                },
                            }
                        },
                        ["auth-redirect"] = {
                            type = "auth",
                            auth = {
                                forward_auth = {
                                    invalid_auth_req_cm_ref = false,
                                    always_set_cookie = false,
                                    url = { "http://", "$host", ":1880", "/auth?id=", "$arg_id", "&code=401&ret_header=aa@bb" },
                                    method = "POST",
                                    auth_headers = { ["My-Custom-Header"] = { "42" } },
                                    auth_request_redirect = { "http://", "$host", "/", "$arg_id" },
                                    upstream_headers = { "aa" },
                                    signin_url = { "http://", "$host", "/signin" }
                                },
                            }
                        },
                        ["auth-fail"] = {
                            type = "auth",
                            auth = {
                                forward_auth = {
                                    invalid_auth_req_cm_ref = false,
                                    always_set_cookie = false,
                                    url = { "http://", "$host", ":1880", "/auth?id=", "$arg_id", "&code=403&ret_header=aa@bb" },
                                    method = "POST",
                                    auth_headers = { ["My-Custom-Header"] = { "42" } },
                                    auth_request_redirect = { "http://", "$host", "/", "$arg_id" },
                                    upstream_headers = { "aa" },
                                    signin_url = {}
                                },
                            }
                        },
                    },
                    backend_group = {
                        { name = "u1", mode = "http", backends = { { address = "127.0.0.1", port = 1880, weight = 100 } }, } }
                }
                ph.set_policy_lua(policy)
                local test_success = function ()
                    local res, err = u.curl("http://127.0.0.1/t1?id=c1&ret_data=1")
                    u.logs("check ", res)
                    h.assert_is_nil(err)
                    local data = common.json_decode(res.body)
                    u.logs("check ", res.body)
                    h.assert_eq(#data, 2)
                    local auth_req = data[1]
                    local real_req = data[2]
                    h.assert_eq(auth_req.uri, "/auth")
                    h.assert_eq(real_req.uri, "/t1")

                    u.logs("check real_req", auth_req.headers)
                    u.logs("check real_res", res.headers)
                    h.assert_eq(auth_req.headers["my-custom-header"], "42")
                    h.assert_eq(auth_req.headers["x-auth-request-redirect"], "http://127.0.0.1/c1")
                    h.assert_eq(auth_req.method, "POST")
                    h.assert_eq(real_req.headers["aa"], "bb")

                    u.logs("check real_res set cookie", res.headers["set-cookie"])
                    h.assert_eq(res.headers["Set-Cookie"], "cc=xx")
                end

                -- auth fail to redirect
                local test_redirect = function ()
                    local res, err = u.curl("http://127.0.0.1/t2?id=c2")
                    h.assert_is_nil(err)
                    u.logs("check ", res.status)
                    h.assert_eq(res.status, 302)
                    h.assert_eq(res.headers["Location"], "http://127.0.0.1/signin")
                end
                -- auth fail without redirect
                local test_auth_fail = function ()
                    local res, err = u.curl("http://127.0.0.1/t3?id=c3")
                    h.assert_is_nil(err)
                    u.logs("check ", res.status, res.headers)
                    h.assert_eq(res.headers["X-ALB-ERR-REASON"], "AuthFail : auth-service-status: 403")
                    h.assert_eq(res.status, 403)
                    return
                end

                local test_auth_with_domain = function ()
                    local res, err = u.curl("http://127.0.0.1/t1?id=c4", { headers = { host = "127.0.0.1" } })
                    h.assert_is_nil(err)
                    u.logs("check ", res.status)
                    u.logs("check ", res.headers)
                    h.assert_eq(res.status, 200, "domain should ok")
                end

                local test_auth_with_https = function ()
                    -- TODO should add success case
                    local res, err = u.curl("http://127.0.0.1/t4?id=c5", { headers = { host = "127.0.0.1" } })
                    h.assert_is_nil(err)
                    u.logs("check ", res.status)
                    u.logs("check ", res.headers)
                    h.assert_eq(res.headers["X-ALB-ERR-REASON"], "AuthFail : send-auth-request-fail")
                    h.assert_eq(res.status, 500)
                end

                test_success()
                test_redirect()
                test_auth_fail()
                test_auth_with_domain()
                test_auth_with_https()
            end
        }
    }
    for i, c in ipairs(cases) do
        u.logs("case " .. i .. ": " .. c.case)
        c.do_test()
    end
end

function _m.test_basic()
    u.logs("basic")
    local policy = {
        certificate_map = {},
        stream = {},
        http = {
            tcp = {
                [80] = {
                    { plugins = { "auth" }, rule = "1", internal_dsl = { { "STARTS_WITH", "URL", "/" } }, config = { refs = { auth = "auth-1" } }, upstream = "u1" },
                }
            }
        },
        config = {
            ["auth-1"] = {
                type = "auth",
                auth = {
                    basic_auth = {
                        auth_type = "basic",
                        realm = "default",
                        err = "",
                        --  openssl passwd -apr1 -salt W60B7kxR bar = $apr1$W60B7kxR$kC.He7pPyJM2io6VH2VNS.
                        secret = {
                            foo = {
                                algorithm = "apr1",
                                salt = "W60B7kxR",
                                hash = "kC.He7pPyJM2io6VH2VNS.",
                            },
                        },
                    },
                }
            },
        },
        backend_group = { { name = "u1", mode = "http", backends = { { address = "127.0.0.1", port = 1880, weight = 100 } }, } }
    }

    ph.set_policy_lua(policy)
    do
        u.logs("without authentication should 401")
        local res, err = u.curl("http://127.0.0.1/echo")
        h.assert_eq(res.status, 401)
        h.assert_eq(res.headers["X-ALB-ERR-REASON"], "AuthFail : basic_auth but req no auth header")
        u.logs(res, err)
    end

    do
        u.logs("invalid passwd should 401")
        local res, err = u.curl("http://127.0.0.1/echo", {
            headers = {
                ["Authorization"] = "Basic not-even-base64=="
            }
        })
        h.assert_eq(res.status, 401)
        h.assert_eq(res.headers["X-ALB-ERR-REASON"], "AuthFail : invalid base64 encoding")
        u.logs(res, err)

        u.logs("invalid passwd should 401")
        -- cspell:disable-next-line
        -- echo "foo:bar-xxx" | base64 = Zm9vOmJhci14eHgK
        local res, err = u.curl("http://127.0.0.1/echo", {
            headers = {
                -- cspell:disable-next-line
                ["Authorization"] = "Basic Zm9vOmJhci14eHgK"
            }
        })
        h.assert_eq(res.status, 401)
        h.assert_eq(res.headers["X-ALB-ERR-REASON"], "AuthFail : invalid user or passwd")
        u.logs(res, err)
    end

    do
        u.logs("valid passwd should 200")
        -- cspell:disable-next-line
        -- echo "foo:bar" | base64 = Zm9vOmJhcg==
        local res, err = u.curl("http://127.0.0.1/echo", {
            headers = {
                -- cspell:disable-next-line
                ["Authorization"] = "Basic Zm9vOmJhcg=="
            }
        })
        u.logs(res, err)
        h.assert_eq(res.status, 200)
        h.assert_eq(res.body, "hello\n")
    end
end

return _m
