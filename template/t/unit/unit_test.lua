local _M={}

local F = require("F");
local u = require("util");
local h = require("test-helper");
function _M.test()
    h.P("in unit test")
    require("unit.replace_prefix_match_test").test()
    require("unit.cert_test").test()
end
return _M