---@diagnostic disable: need-check-nil
local _M = {}
local u = require "util"
local h = require "test-helper"

function _M.as_backend()
    u.logs(ngx.var.uri)
    if string.find(ngx.var.uri, "404-without-body", 1, true) then
        u.logs("404-without-body ====", ngx.var.uri)
        ngx.status = 404
        ngx.exit(ngx.HTTP_OK)
    end
    if string.find(ngx.var.uri, "404") then
        u.logs("expect x 404 ====", ngx.var.uri)
        ngx.status = 404
        ngx.say("im 404")
        ngx.exit(ngx.HTTP_OK)
    end
    if string.find(ngx.var.uri, "504") then
        ngx.status = 504
        ngx.exit(ngx.HTTP_OK)
    end

    if string.find(ngx.var.uri, "sleep") then
        local t = tonumber(ngx.var.arg_sleep)
        u.logs("sleep in backend start", t)
        ngx.sleep(t)
        u.logs("sleep in backend over", t)
        ngx.status = 502
        ngx.say("im 502")
        ngx.exit(ngx.HTTP_OK)
    end
end

function _M.test()
    -- LuaFormatter off
    local policy = {
        http = {tcp = {["80"] = {
            {rule = "1", internal_dsl = {{"STARTS_WITH", "URL", "/t1"}}, upstream = "u1", config = {timeout = {proxy_read_timeout_ms = "300"}}}}}
        },
        backend_group = {
            {name = "u1", mode = "http", backends = {{address = "127.0.0.1", port = 1880, weight = 100}}, }}
        }
    -- LuaFormatter on
    require("policy_helper").set_policy_lua(policy)
    do
        u.logs "error from backend without body"
        local res, err = u.curl("http://127.0.0.1/t1/404-without-body")
        u.logs(res, err)
        h.assert_eq(res.status,404)
        h.assert_eq(res.body, "")
    end

    do
        u.logs "error from alb itself"
        local res, err = u.curl("http://127.0.0.1/xx")
        u.logs(res, err)
        h.assert_eq(res.status, 404)
    end

    do
        u.logs "error from alb timeout"
        local res, err = u.curl("http://127.0.0.1/t1/sleep?sleep=5")
        u.logs(res, err)
        h.assert_eq(res.status, 504)
        h.assert_eq(res.body, "X-Error: 504\n")
    end

    do
        u.logs "error from backend timeout"
        local res, err = u.curl("http://127.0.0.1/t1/504")
        u.logs(res, err)
        h.assert_eq(res.status, 504)
        h.assert_eq(res.body, "")
    end

end

return _M
