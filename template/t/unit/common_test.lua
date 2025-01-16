local _M = {}

local t = require("test-helper");
local u = require("util");
local c = require("utils.common")

function _M.test()
    u.logs("in common test")
    u.logs(c.json_decode('{"a":null}'))
end

return _M
