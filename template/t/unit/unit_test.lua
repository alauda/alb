local _M = {}

local u = require "util"
local h = require("test-helper");

function _M.test()
    u.logs("in unit test")
    local run_only = os.getenv("ALB_LUA_UNIT_TEST_CASE")
    if run_only and run_only ~= "" then
        u.logs("run only", run_only)
        require(run_only).test()
        return
    end
    require("unit.replace_prefix_match_test").test()
    require("unit.cert_test").test()
    require("unit.cors_test").test()
    require("unit.common_test").test()
    require("unit.plugins.auth.auth_unit_test").test()
end

return _M
