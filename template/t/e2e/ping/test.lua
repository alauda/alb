-- format:on
local _M = {}
local h = require "test-helper"
local u = require "util"

function _M.as_backend()
    ngx.say "ok"
end

function _M.test()
    require("policy_helper").set_policy_lua({http = {tcp = {["80"] = {{rule = "1", internal_dsl = {{"STARTS_WITH", "URL", "/t1"}}, upstream = "test-upstream-1"}}}}, backend_group = {{name = "test-upstream-1", mode = "http", backends = {{address = "127.0.0.1", port = 1880, weight = 100}}}}})

    local res, err = require"resty.http".new():request_uri("http://127.0.0.1:80/t1", {})
    u.logs(res, err)
    h.assert_eq(res.body, "ok\n")
end

return _M
