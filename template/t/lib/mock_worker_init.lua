-- format:on
local M = {}

local balancer = require("balancer.balance")
local ph = require("config.policy_fetch")
local cache = require("config.cache")
local u = require("util")
local subsys = require "utils.subsystem"
local shm = require "config.shmap"

function M.init_worker(_cfg)
    ngx.update_time()
    u.log("life: init worker " .. tostring(ngx.worker.id()))
    if subsys.is_http_subsystem() then
        cache.init_l7()
    else
        cache.init_l4()
    end
    local policy_raw, err = u.file_read_to_string(os.getenv("TEST_BASE") .. "/policy.new")
    if err ~= nil then
        ngx.exit(0)
    end
    ngx.update_time()
    shm.set_policy_raw("{}")
    ngx.update_time()
    ph.update_policy(policy_raw, "manual")
    ngx.update_time()
    u.log "life: update policy ok"
    balancer.sync_backends()
    ngx.update_time()
    u.log "life: sync backend ok"
    if subsys.is_http_subsystem() then
        ngx.update_time()
        require("metrics").init()
        ngx.update_time()
        u.log "life: init metrics ok"
    end
end

return M
