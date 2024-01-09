local M = {}

local u = require "util"

function M.init_worker()
    u.log "init worker"
    local subsys = require "utils.subsystem"
    if subsys.is_http_subsystem() then
        require "init_l7"
    else
        require "init_l4"
    end
    local balancer = require "balancer"
    balancer.sync_backends()
    u.log "sync backend ok"
    local _, err = ngx.timer.every(1, balancer.sync_backends)
    if err then
        ngx.log(ngx.ERR, string.format("error when setting up timer.every for sync_backends: %s", tostring(err)))
    end
end
return M