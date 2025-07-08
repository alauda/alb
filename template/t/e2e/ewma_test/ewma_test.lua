-- format:on
local _M = {}
local h = require "test-helper"
local u = require "util"
local ph = require("policy_helper")

function _M.as_backend()
    if ngx.var.uri == "/t3" then
        ngx.sleep(1)
        ngx.exit(504)
    else
        ngx.say "ok"
    end
end

function _M.test()
    -- LuaFormatter off
    ph.set_policy_lua({
        http = {
            tcp = {
                ["80"] = {
                    { rule = "1", internal_dsl = { { "STARTS_WITH", "URL", "/t1" } }, upstream = "test-upstream-1", },
                    { rule = "2", internal_dsl = { { "STARTS_WITH", "URL", "/t2" } }, upstream = "test-upstream-2", }, 
                    {
                         rule = "3", internal_dsl = { { "STARTS_WITH", "URL", "/t3" } }, upstream = "test-upstream-3",
                         plugins = { "timeout" },
                         config = { refs = { timeout = "timeout-3" } },
                    },
                }
            }
        },
        backend_group = {
            {
                name = "test-upstream-1",
                mode = "http",
                session_affinity_policy = "ewma",
                session_affinity_attribute = '{"pick_set_size": 2, "decay_time": 3}',
                backends = {
                    { address = "127.0.0.1", port = 1880, weight = 100 },
                }
            },
            {
                name = "test-upstream-2",
                mode = "http",
                session_affinity_policy = "ewma",
                backends = {
                    { address = "127.0.0.1", port = 1881, weight = 100 }, -- sth not exist
                    { address = "127.0.0.1", port = 1882, weight = 100 }, -- sth not exist
                    { address = "127.0.0.1", port = 1883, weight = 100 }, -- sth not exist
                }
            },
            {
                name = "test-upstream-3",
                mode = "http",
                session_affinity_policy = "ewma",
                backends = {
                    { address = "127.0.0.1", port = 1880, weight = 100 },
                    { address = "127.0.0.1", port = 1880, weight = 100 },
                    { address = "127.0.0.1", port = 1880, weight = 100 },
                }
            }
        },
    config = {
        ["timeout-3"] = {
            type = "timeout",
            timeout = {
                proxy_read_timeout_ms = 100,
            }
        },
    },
    }
    )
    -- LuaFormatter on
    h.assert_curl("http://127.0.0.1:80/t1?v=v", {}, { status = 200 })
    h.assert_curl("http://127.0.0.1:80/t2?v=v", {}, { status = 502 })
    h.assert_curl("http://127.0.0.1:80/t3?v=v", {}, { status = 504 })
end

return _M
