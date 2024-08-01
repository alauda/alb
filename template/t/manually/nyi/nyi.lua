local ph = require "policy_helper"
local u = require "util"
local h = require "test-helper"
local _M = {}

local function get_nyi_log()
    local cmd = string.format([[bash -c "cat %s |grep 'NYI' " ]], "./template/.nyi.log")
    local out, err = u.shell(cmd)
    if err ~= nil and err ~= "" then
        h.fail("get nyi log fail " .. err)
    end
    return out
end
function _M.test()
    ph.set_policy_lua({
        http = {
            tcp = {
                ["80"] = {
                    { rule = "fortio", internal_dsl = { { "STARTS_WITH", "URL", "/" } }, upstream = "fortio" },
                }
            }
        },
        backend_group = {
            { name = "fortio", mode = "http", backends = { { address = "127.0.0.1", port = 8080, weight = 100 } } }
        },
    })
    u.logs("hello")
    local time = "30s"
    local cmd =
        "fortio load -a -labels 'u:nyi' -logger-force-color -c 8 -qps 30000 -nocatchup -uniform -t " ..
        time .. " 'http://127.0.0.1:80?size=1024:99'"
    local out, err = u.shell(cmd)
    u.logs("out", out, "err", err)
    u.log(get_nyi_log())
end

return _M
