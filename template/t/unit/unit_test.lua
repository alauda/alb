local _M = {}

local h = require("test-helper");
function _M.test()
    h.P("in unit test")
    require("unit.replace_prefix_match_test").test()
    require("unit.cert_test").test()
    require("unit.cors_test").test()
end

return _M
