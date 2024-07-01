local _M = {}

local F = require("F");local u = require("util");local h = require("test-helper");
function _M.test()
    u.log(F"case test")
    ngx.sleep(4000)
end
return _M