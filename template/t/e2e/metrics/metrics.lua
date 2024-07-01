-- format:on
local _M = {}
local h = require "test-helper"
local u = require "util"
local sext = require "utils.string_ext"

function _M.as_backend()
    ngx.say "ok"
end

function _M.test()
    require("policy_helper").set_policy_lua({http = {tcp = {["80"] = {{rule = "r1", internal_dsl = {{"STARTS_WITH", "URL", "/t1"}}, upstream = "test-upstream-1"}}}}, backend_group = {{name = "test-upstream-1", mode = "http", backends = {{address = "127.0.0.1", port = 1880, weight = 100}}}}})

    local res = h.assert_curl("http://127.0.0.1:80/t1")
    h.assert_eq(res.body, "ok\n")

    local metrics = h.assert_curl("http://127.0.0.1:1936/metrics")
    local status = sext.lines_grep(metrics.body, [[nginx_http_status{port="80",rule="r1]])
    u.logs("init", status)

    h.assert_curl("http://127.0.0.1:1936/clear")

    local metrics = h.assert_curl("http://127.0.0.1:1936/metrics")
    local status = sext.lines_grep(metrics.body, [[nginx_http_status{port="80",rule="r1]])
    u.logs("after clear", status)

    h.assert_eq(#status, 0)
end

return _M
