local mlcache = require "resty.mlcache"
local ngx_config = ngx.config
local subsystem = ngx_config.subsystem
local channel_name = "mlcache"
local common = require "utils.common"
local shm = require "config.shmap"
local ev = require "vendor.events"

local _M = {}

function _M.init_mlcache(name, shared_dict, opt)
    local c, err = mlcache.new(name, shared_dict, opt)
    if not c then
        ngx.log(ngx.ERR, "create mlcache failed, " .. err)
    end
    _M[name] = c
end

local ipc = {
    register_listeners = function (events)
        for _, event_t in pairs(events) do
            ev.register(function (data)
                event_t.handler(data)
            end, channel_name, event_t.channel)
        end
    end,
    broadcast = function (channel, data)
        local ok, err = ev.post(channel_name, channel, data)
        if not ok then
            ngx.log(ngx.ERR, "failed to post event '", channel_name, "', '", channel, "': ", err)
        end
    end,
    poll = function (timeout) -- luacheck: ignore
        return ev.poll()
    end
}

function _M.init_l4()
    local cache = _M
    local ok, err = ev.configure {
        shm = subsystem .. "_ipc_shared_dict", -- defined by "lua_shared_dict"
        timeout = 5,                           -- life time of event data in shm
        interval = 1,                          -- poll interval (seconds)

        wait_interval = 0.010,                 -- wait before retry fetching event data
        wait_max = 0.5                         -- max wait time before discarding event
    }
    if not ok then
        ngx.log(ngx.ERR, "failed to start event system: ", err)
        return
    end
    cache.init_mlcache("rule_cache", subsystem .. "_alb_cache", { lru_size = 2000, ttl = 30, neg_ttl = 5, ipc = ipc })
end

function _M.init_l7()
    local cache = _M
    local ok, err = ev.configure {
        shm = subsystem .. "_ipc_shared_dict", -- defined by "lua_shared_dict"
        timeout = 5,                           -- life time of event data in shm
        interval = 1,                          -- poll interval (seconds)

        wait_interval = 0.010,                 -- wait before retry fetching event data
        wait_max = 0.5                         -- max wait time before discarding event
    }
    if not ok then
        ngx.log(ngx.ERR, "failed to start event system: ", err)
        return
    end

    cache.init_mlcache("rule_cache", subsystem .. "_alb_cache", { lru_size = 2000, ttl = 60, neg_ttl = 5, ipc = ipc })

    cache.init_mlcache("cert_cache", subsystem .. "_alb_cache", { lru_size = 500, ttl = 60, neg_ttl = 5, ipc = ipc })

    cache.init_mlcache("config_cache", subsystem .. "_alb_cache", { lru_size = 500, ttl = 60, neg_ttl = 5, ipc = ipc })
end

---gen_rule_key
---@param subsystem "stream"|"http"
---@param protocol  "tcp"|"udp"
---@param port number
---@return string
function _M.gen_rule_key(subsystem, protocol, port)
    return string.format("%s_%s_%d", subsystem, string.lower(protocol), port)
end

function _M.remove_config(key)
    local cache = _M
    cache.config_cache:delete(key)
end

local function get_config_from_shdict(key)
    ngx.log(ngx.INFO, "cache: refresh cache from ngx.share[http_policy][" .. tostring(key) .. "]")
    local raw = shm.get_config(key)
    if raw == nil then
        return nil, string.format("no %s", key)
    end
    return common.json_decode(raw), nil
end

---@param key string
---@return RefBox config
---@return string error
function _M.get_config(key)
    local cache = _M
    cache.config_cache:update(0.1)
    local config, err, _ = cache.config_cache:get(key, nil, get_config_from_shdict, key)
    return config, err
end

--- @class RefedConfig
--- @field hash string
--- @field config any?

---@param policy Policy
---@param kind string
---@return RefedConfig? config
---@return string? error
function _M.get_refed_config(policy, kind)
    local config = policy.config
    if config == nil then
        return nil, nil
    end
    local key = config.refs[kind]
    if key == nil then
        return nil, nil
    end
    local cfg, err = _M.get_config(key)
    if err ~= nil then
        return nil, err
    end
    return { config = cfg[kind], hash = key }, nil
end

---@param policy Policy
---@param kind string
---@return any? config
---@return any? error
function _M.get_config_from_policy(policy, kind)
    local config = policy.config
    if config == nil then
        return nil
    end
    if config[kind] ~= nil then
        return config[kind]
    end
    local key = config.refs[kind]
    if key == nil then
        return nil
    end
    local cfg, err = _M.get_config(key)
    if err ~= nil then
        return nil, err
    end
    return cfg[kind], nil
end

return _M
