-- format:on
local M = {}

local u = require("util")
local balancer = require("balancer.balance")
local cache = require("config.cache")

function M.init_worker()
    u.log("init worker " .. tostring(ngx.worker.id()))
    local subsys = require "utils.subsystem"
    if subsys.is_http_subsystem() then
        cache.init_l7()
    else
        cache.init_l4()
    end
    balancer.sync_backends()
    u.log "sync backend ok"
    local _, err = ngx.timer.every(1, balancer.sync_backends)
    if err then
        ngx.log(ngx.ERR, string.format("error when setting up timer.every for sync_backends: %s", tostring(err)))
    end

    if subsys.is_http_subsystem() then
        require("metrics").init()
    end
end

return M
