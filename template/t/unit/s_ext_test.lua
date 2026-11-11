local _M = {}

local h = require("test-helper");
local u = require("util")
local s_ext = require("utils.string_ext")

function _M.test()
    local ips = s_ext.split("127.0.0.1,127.0.0.2", ",")
    u.logs(ips)
    h.assert_eq(ips[1], "127.0.0.1")
    h.assert_eq(ips[2], "127.0.0.2")
    local ips = s_ext.split(",127.0.0.2,", ",")
    u.logs(ips)
    local ips = s_ext.split("    ,127.0.0.2,", ",")
    u.logs(ips)
    local ips = s_ext.split("127.0.0.1",",")
    u.logs(ips)
    local ips = s_ext.split("",",")
    u.logs(ips)
end

return _M
