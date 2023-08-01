local _M = {}

local F = require("F");
local u = require("util")
local h = require("test-helper");
local r = require("replace_prefix_match")

-- return the new url
function _M.test()
    local cases = {
        {"/foo/bar", "/foo", "/xyz", "/xyz/bar"}, --
        {"/foo/bar", "/foo", "/xyz/", "/xyz/bar"}, --
        {"/foo/bar", "/foo/", "/xyz", "/xyz/bar"}, --
        {"/foo/bar", "/foo/", "/xyz/", "/xyz/bar"}, --
        {"/foo", "/foo", "/xyz", "/xyz"}, --
        {"/foo/", "/foo", "/xyz", "/xyz/"}, --
        {"/foo/bar", "/foo", "", "/bar"}, --
        {"/foo/", "/foo", "", "/"}, --
        {"/foo", "/foo", "", "/"}, --
        {"/foo/", "/foo", "/", "/"}, --
        {"/foo", "/foo", "/", "/"} --
    }
    local fail = false
    local msg = ""
    for k, c in pairs(cases) do
        h.P(F "case {k}")
        local url = c[1]
        local prefix_match = c[2]
        local replace_prefix = c[3]
        local expect_result = c[4]
        local result = r.replace(url, prefix_match, replace_prefix)
        if expect_result ~= result then
            h.P(F "fail {k} e {expect_result} r {result} " .. u.inspect(c))
            fail = true
            break
        end
    end
    if fail then
        h.P("fail ")
        h.fail()
    end
end
return _M
