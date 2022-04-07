local cache = require "cache"
local ngx_config = ngx.config
local subsystem = ngx_config.subsystem
local ev = require "vendor.events"

local ok, err = ev.configure {
    shm = subsystem .. "_ipc_shared_dict", -- defined by "lua_shared_dict"
    timeout = 5,            -- life time of event data in shm
    interval = 1,           -- poll interval (seconds)

    wait_interval = 0.010,  -- wait before retry fetching event data
    wait_max = 0.5,         -- max wait time before discarding event
}
if not ok then
    ngx.log(ngx.ERR, "failed to start event system: ", err)
    return
end
local channel_name = "mlcache"

cache.init_mlcache("rule_cache", subsystem .. "_alb_cache", {
    lru_size = 2000,
    ttl      = 60,
    neg_ttl  = 5,
    ipc = {
        register_listeners = function(events)
            for _, event_t in pairs(events) do
                ev.register(function(data)
                    event_t.handler(data)
                end, channel_name, event_t.channel)
            end
        end,
        broadcast = function(channel, data)
            local ok, err = ev.post(channel_name, channel, data)
            if not ok then
                ngx.log(ngx.ERR, "failed to post event '", channel_name, "', '",
                        channel, "': ", err)
            end
        end,
        poll = function(timeout) --luacheck: ignore
            return ev.poll()
        end,
    },
})

cache.init_mlcache("cert_cache", subsystem .. "_alb_cache", {
    lru_size = 500,
    ttl      = 60,
    neg_ttl  = 5,
    ipc = {
        register_listeners = function(events)
            for _, event_t in pairs(events) do
                ev.register(function(data)
                    event_t.handler(data)
                end, channel_name, event_t.channel)
            end
        end,
        broadcast = function(channel, data)
            local ok, err = ev.post(channel_name, channel, data)
            if not ok then
                ngx.log(ngx.ERR, "failed to post event '", channel_name, "', '",
                        channel, "': ", err)
            end
        end,
        poll = function(timeout) --luacheck: ignore
            return ev.poll()
        end,
    },
})
