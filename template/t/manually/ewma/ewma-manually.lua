-- format:on
local _M = {}
local h = require "test-helper"
local u = require "util"
local ph = require("policy_helper")

function _M.as_backend()
    ngx.say "ok"
end

function _M.test()
    -- LuaFormatter off
    ph.set_policy_lua({
        http = {
            tcp = {
                ["80"] = {
                    { rule = "1", internal_dsl = { { "STARTS_WITH", "URL", "/t1" } }, upstream = "test-upstream-1", } }
            }
        },
        backend_group = {
            {
                name = "test-upstream-1",
                mode = "http",
                session_affinity_policy = "ewma",
                backends = {
                    { address = "172.18.0.2", port = 8000, weight = 100 },
                    { address = "172.18.0.2", port = 8001, weight = 100 },
                    { address = "172.18.0.2", port = 8002, weight = 100 },
                }
            }
        }
    }
    )
    -- LuaFormatter on
    h.assert_curl("http://127.0.0.1:80/t1?v=v", {}, { status = 200 })
    ngx.sleep(1000)
end

return _M
