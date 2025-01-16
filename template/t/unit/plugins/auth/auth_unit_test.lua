-- format:on style:emmy
local _M = {}
local F = require("F");
local u = require("util")
local h = require("test-helper");
local str = require "resty.string"
local auth = require("plugins.auth.auth")
local crypt = require "plugins.auth.crypt"
local forward_auth = require("plugins.auth.forward_auth")

function _M.test()
    u.logs("in auth unit test")
    _M.test_resolve_varstring()
    _M.test_merge_cookie()

    -- -- 实际上不能处理t=$xx这种语句
    u.logs("test var ", ngx.var["t=${msec}"], ngx.var["msec"])
    _M.test_basic_auth()
    _M.test_simple_apr1_perf()
end

function _M.test_resolve_varstring()
    local var = {
        uri = "a.com/1",
        header_z = "zzz"
    }
    local cases = {
        {
            arg = { "http://", "$uri", "/", "$header_z" },
            expect = "http://a.com/1/zzz"
        }
    }
    for k, c in pairs(cases) do
        local result, err = forward_auth.resolve_varstring(c.arg, var)
        u.logs(c, result, err)
        if err then
            h.P(F "fail {k} e {c.expect} r {result} " .. u.inspect(c))
            h.fail()
        end
        if c.expect ~= result then
            h.P(F "fail {k} e {c.expect} r {result} " .. u.inspect(c))
            h.fail()
        end
    end
end

function _M.test_merge_cookie()
    local cases = {
        {
            arg = { nil, nil },
            expect = nil
        },
        {
            arg = { "a", nil },
            expect = "a"
        },
        {
            arg = { "a", "b" },
            expect = { "a", "b" }
        },
        {
            arg = { { "a" }, "b" },
            expect = { "a", "b" }
        },
        {
            arg = { { "a" }, { "b" } },
            expect = { "a", "b" }
        },
        {
            arg = { { "a" }, { "b", "c" } },
            expect = { "a", "b", "c" }
        }
    }
    for k, c in pairs(cases) do
        local result = forward_auth.merge_cookie(c.arg[1], c.arg[2])
        u.logs("x", k, c.expect, "r", result, "case", c.arg)
        h.assert_eq(result, c.expect)
    end
end

function _M.test_simple_apr1_perf()
    local t_cfg = os.getenv("ALB_LUA_TEST_CFG") or ""
    local apr1 = string.find(t_cfg, "apr1", 1, true)
    if not apr1 then
        return
    end
    local keep_run = string.find(t_cfg, "flamegraph", 1, true)
    while true do
        ngx.update_time()
        local s = ngx.now()
        local n = 10000
        for i = 1, n, 1 do
            crypt.apr1("bar", "W60B7kxR")
        end
        ngx.update_time()
        local e = ngx.now()
        ngx.log(ngx.INFO, "time " .. tostring(n) .. " " .. (e - s) .. " qps " .. 1 * n / (e - s),
            " 1 call " .. tostring((e - s) / n * 1000) .. "ms")
        -- ngx.sleep(3)
        if not keep_run then
            break
        end
    end
end

function _M.test_basic_auth()
    local cases = {
        {
            pass = "bar",
            slat = "W60B7kxR",
            hash = "kC.He7pPyJM2io6VH2VNS."
        },
        {
            pass = "%^&*",
            slat = "WEm2C/nC",
            -- cspell:disable-next-line
            hash = "MjXcOZacoKaDjPuE0.Xyc."
        },
        {
            pass = "1a2b3c%^&*()_+",
            slat = "FhnHptBM",
            hash = "4HP5UXIwuVHSvhZr/o96s."
        }
    }
    -- test via  `openssl passwd -apr1 -salt W60B7kxR bar` or
    for k, c in pairs(cases) do
        local result = crypt.apr1(c.pass, c.slat)
        u.logs("x", k, c.hash, "r", result, #c.hash, #result)
        h.assert_eq(result, c.hash)
    end
end

return _M
