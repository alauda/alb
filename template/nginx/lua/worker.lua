-- format:on
local p = require "policy_fetch"

local subsys = require "utils.subsystem"
local string_format = string.format
local ngx = ngx

local balancer = require "balancer"
local ngx_log = ngx.log
local ngx_timer = ngx.timer
local ngx_worker = ngx.worker
local sync_policy_interval = tonumber(os.getenv("SYNC_POLICY_INTERVAL"))
local clean_metrics_interval = tonumber(os.getenv("CLEAN_METRICS_INTERVAL"))

-- init cache in each worker
if subsys.is_http_subsystem() then
    require "init_l7"
else
    require "init_l4"
end

if ngx_worker.id() == 0 then
    -- master sync policy
    p.fetch_policy()
    local _, err = ngx_timer.every(sync_policy_interval, p.fetch_policy)
    if err then
        ngx_log(ngx.ERR, string_format("error when setting up timer.every for fetch_policy: %s", tostring(err)))
    end
end

-- sync backend cache in each worker
balancer.sync_backends()
local _, err = ngx_timer.every(sync_policy_interval, balancer.sync_backends)
if err then
    ngx_log(ngx.ERR, string_format("error when setting up timer.every for sync_backends: %s", tostring(err)))
end

local clean_metrics = function(premature)
    if premature then
        return
    end
    if subsys.is_http_subsystem() then
        require("metrics").clear()
    end
end

-- worker clean up metrics data for prometheus periodically
if ngx_worker.id() == 0 then
    -- master clean up metrics data
    local _, err = ngx_timer.every(clean_metrics_interval, clean_metrics)
    if err then
        ngx_log(ngx.ERR, string_format("error when setting up timer.every for clean_metrics: %s", tostring(err)))
    end
end
