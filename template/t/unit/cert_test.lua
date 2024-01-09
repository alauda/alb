-- format:on
local _M = {}

local F = require("F");
local u = require("util")
local h = require("test-helper");
local ct = require("cert_tool")

function _M.test()

    -- LuaFormatter off
    local cert_map = {
        ["a.com"] = "full-host",
        ["b.com/443"] = "full-host-withport-443",
        ["b.com/8443"] = "full-host-withport-8443",
        ["10443"] = "10443-default",
        ["*.e.com"] = "wildcard-host",
        ["*.f.com/443"] = "wildcard-host-443"
        }
    ct.get_cert = function(domain)
        local cert = cert_map[domain]
        if cert ~= nil then
            return cert, true
        end
        return nil, false
    end
    local cases = {
        {"a.com", "443", "full-host"},
        {"a.com", nil, "full-host"},
        {nil,"10443", "10443-default"},
        {nil,nil, nil},
        {"a.com", "8443", "full-host"},
        {"b.com", "443", "full-host-withport-443"},
        {"b.com", "8443", "full-host-withport-8443"},
        {"b.com", "9443", nil},
        {"b.com", "10443", "10443-default"},
        {"c.com", "443", nil},
        {"a.e.com", "443", "wildcard-host"},
        {"c.e.com", "443", "wildcard-host"},
        {"c.e.com", "8443", "wildcard-host"},
        {"a.f.com", "8443", nil},
        {"a.f.com", "443", "wildcard-host-443"},
    }
    -- LuaFormatter on
    for k, c in pairs(cases) do
        h.P(F "case {k}")
        local domain = c[1]
        local port = c[2]
        local expect_result = c[3]
        local result = ct.try_get_domain_cert_from_l2_cache(domain, port)
        if expect_result ~= result then
            h.P(F "fail {k} e {expect_result} r {result} " .. u.inspect(c))
            h.fail()
        end
    end
end
return _M
