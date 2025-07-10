-- take from https://github.com/kubernetes/ingress-nginx/blob/main/rootfs/etc/nginx/lua/balancer/ewma.lua
-- Original Authors: Shiv Nagarajan & Scott Francis
-- Accessed: March 12, 2018
-- Inspiration drawn from:
-- https://github.com/twitter/finagle/blob/1bc837c4feafc0096e43c0e98516a8e1c50c4421
--   /finagle-core/src/main/scala/com/twitter/finagle/loadbalancer/PeakEwma.scala


local resty_lock = require("resty.lock")
local util = require("balancer.ewma.ewma_util")
local split = require("balancer.ewma.split")
local common = require("utils.common")
local peer_map_helper = require "balancer.peer_map_helper"
local bc = require("balancer.common")

local ngx = ngx
local math = math
local pairs = pairs
local ipairs = ipairs
local tostring = tostring
local tonumber = tonumber
local setmetatable = setmetatable
local string_format = string.format
local table_insert = table.insert
local ngx_log = ngx.log
local INFO = ngx.INFO

local DEFAULT_DECAY_TIME = tonumber(os.getenv("EWMA_DECAY_TIME")) or 10 -- 10s:36% 15s:22% 20s:13% 30s:5%
local DEFAULT_PICK_SET_SIZE = tonumber(os.getenv("EWMA_PICK_SET_SIZE")) or 2
local ENABLE_RETRY_RTT_THRESHOLD = os.getenv("EWMA_ENABLE_RETRY_RTT_THRESHOLD") or true
-- response time maybe 0 when retry, avoid count it as a low score
local DEFAULT_RETRY_RTT_THRESHOLD = tonumber(os.getenv("EWMA_RETRY_RTT_THRESHOLD")) or 0.3 -- 300ms it should be the average rtt
local LOCK_KEY = ":ewma_key"

ngx.log(ngx.INFO,
    "ewma default decay time: " ..
    tostring(DEFAULT_DECAY_TIME) .. " default pick set size: " .. tostring(DEFAULT_PICK_SET_SIZE))

-- local EWMA_DEBUG = os.getenv("EWMA_DEBUG") or false
-- local function ewma_log(msg)
--     if EWMA_DEBUG then
--         ngx.log(ngx.WARN, "[ewma-debug] ", tostring(msg))
--     end
-- end

local ewma_lock, ewma_lock_err = resty_lock:new("balancer_ewma_locks", { timeout = 0, exptime = 0.1 })
if not ewma_lock then
    error(ewma_lock_err)
end

local _M = { name = "ewma" }


--- @class EwmaConfig
--- @field pick_set_size number
--- @field decay_time number
--- @field retry_rtt_threshold number




---@param backend BackendGroup
---@return EwmaConfig
local function parse_ewma_config(backend)
    local default_config = {
        pick_set_size = DEFAULT_PICK_SET_SIZE,
        decay_time = DEFAULT_DECAY_TIME,
        retry_rtt_threshold = DEFAULT_RETRY_RTT_THRESHOLD,
    }
    if backend.session_affinity_attribute == nil then
        return default_config
    end
    local config = common.json_decode(backend.session_affinity_attribute)
    if config == nil then
        return default_config
    end
    return {
        pick_set_size = config.pick_set_size or default_config.pick_set_size,
        decay_time = config.decay_time or default_config.decay_time,
        retry_rtt_threshold = config.retry_rtt_threshold or default_config.retry_rtt_threshold,
    }
end

local function lock(upstream)
    local _, err = ewma_lock:lock(upstream .. LOCK_KEY)
    if err then
        if err ~= "timeout" then
            ngx.log(ngx.ERR, string_format("EWMA Balancer failed to lock: %s", tostring(err)))
        end
    end

    return err
end

local function unlock()
    local ok, err = ewma_lock:unlock()
    if not ok then
        ngx.log(ngx.ERR, string_format("EWMA Balancer failed to unlock: %s", tostring(err)))
    end

    return err
end

local function decay_ewma(ewma, last_touched_at, rtt, now, decay_time)
    local td = now - last_touched_at
    td = (td > 0) and td or 0
    local weight = math.exp(-td / decay_time)

    ewma = ewma * weight + rtt * (1.0 - weight)
    return ewma
end

local function store_stats(upstream, ewma, now)
    local success, err, forcible = ngx.shared.balancer_ewma_last_touched_at:set(upstream, now)
    if not success then
        ngx.log(ngx.WARN, "balancer_ewma_last_touched_at:set failed " .. err)
    end
    if forcible then
        ngx.log(ngx.WARN, "balancer_ewma_last_touched_at:set valid items forcibly overwritten")
    end

    success, err, forcible = ngx.shared.balancer_ewma:set(upstream, ewma)
    if not success then
        ngx.log(ngx.WARN, "balancer_ewma:set failed " .. err)
    end
    if forcible then
        ngx.log(ngx.WARN, "balancer_ewma:set valid items forcibly overwritten")
    end
end

---@param upstream string
---@param rtt number
---@param update boolean
---@param decay_time number
---@return number, string|nil
local function get_or_update_ewma(upstream, rtt, update, decay_time)
    local lock_err = nil
    if update then
        lock_err = lock(upstream)
    end
    local ewma = ngx.shared.balancer_ewma:get(upstream) or 0
    if lock_err ~= nil then
        return ewma, lock_err
    end

    local now = ngx.now()
    local last_touched_at = ngx.shared.balancer_ewma_last_touched_at:get(upstream) or 0
    local ewma_old = ewma
    ewma = decay_ewma(ewma_old, last_touched_at, rtt, now, decay_time)
    -- ewma_log("get_or_update_ewma " ..
    --     "update " ..
    --     tostring(update) ..
    --     " "..
    --     tostring(upstream) ..
    --     " score " ..
    --     tostring(ewma_old) ..
    --     " > " ..
    --     tostring(ewma) ..
    --     " rtt " .. tostring(rtt) .. " now " .. tostring(now) .. " td " .. tostring(now - last_touched_at))

    if not update then
        return ewma, nil
    end

    store_stats(upstream, ewma, now)

    unlock()

    return ewma, nil
end

---@param upstream Backend
local function get_upstream_name(upstream)
    return bc.node_key(upstream)
end


local function score(upstream, decay_time)
    -- Original implementation used names
    -- Endpoints don't have names, so passing in IP:Port as key instead
    local upstream_name = get_upstream_name(upstream)
    return get_or_update_ewma(upstream_name, 0, false, decay_time)
end

-- implementation similar to https://en.wikipedia.org/wiki/Fisher%E2%80%93Yates_shuffle
-- or https://en.wikipedia.org/wiki/Random_permutation
-- loop from 1 .. k
-- pick a random value r from the remaining set of unpicked values (i .. n)
-- swap the value at position i with the value at position r
local function shuffle_peers(peers, k)
    for i = 1, k do
        local rand_index = math.random(i, #peers)
        peers[i], peers[rand_index] = peers[rand_index], peers[i]
    end
    -- peers[1 .. k] will now contain a randomly selected k from #peers
end

---@param peers Backend[]
---@param k number
---@param config EwmaConfig
---@return Backend, number
local function pick_and_score(peers, k, config)
    shuffle_peers(peers, k)
    local lowest_score_index = 1
    local lowest_score = score(peers[lowest_score_index], config.decay_time)
    -- ewma_log("pick_and_score ep index " .. tostring(lowest_score_index) .. " ep " ..
    --     get_upstream_name(peers[lowest_score_index]) .. " score " .. tostring(lowest_score))
    for i = 2, k do
        local new_score = score(peers[i], config.decay_time)
        -- ewma_log("pick_and_score index " .. tostring(i) .. " ep " .. get_upstream_name(peers[i]) .. " score " ..
        --     tostring(new_score))
        if new_score < lowest_score then
            lowest_score_index, lowest_score = i, new_score
        end
    end

    return peers[lowest_score_index], lowest_score
end

-- slow_start_ewma is something we use to avoid sending too many requests
-- to the newly introduced endpoints. We currently use average ewma values
-- of existing endpoints.
local function calculate_slow_start_ewma(self)
    local total_ewma = 0
    local endpoints_count = 0

    for _, endpoint in pairs(self.peers) do
        local endpoint_string = get_upstream_name(endpoint)
        local ewma = ngx.shared.balancer_ewma:get(endpoint_string)

        if ewma then
            endpoints_count = endpoints_count + 1
            total_ewma = total_ewma + ewma
        end
    end

    if endpoints_count == 0 then
        ngx.log(ngx.INFO, "no ewma value exists for the endpoints")
        return nil
    end

    return total_ewma / endpoints_count
end

function _M.balance(self)
    local peers = self.peers
    local endpoint, ewma_score = peers[1], -1
    local pick_set_size = self.config.pick_set_size

    if #peers > 1 then
        local k = (#peers < pick_set_size) and #peers or pick_set_size

        local tried_endpoints
        if not ngx.ctx.balancer_ewma_tried_endpoints then
            tried_endpoints = {}
            ngx.ctx.balancer_ewma_tried_endpoints = tried_endpoints
        else
            tried_endpoints = ngx.ctx.balancer_ewma_tried_endpoints
        end
        local now = ngx.now()
        local last_tried_endpoint = ngx.ctx.balancer_ewma_tried_endpoint_last
        local last_tried_time = now - (ngx.ctx.balancer_ewma_tried_endpoint_time or now)
        if last_tried_endpoint and ENABLE_RETRY_RTT_THRESHOLD then
            -- ewma_log("update ewma of retry ep "..last_tried_endpoint.." last_tried_time "..last_tried_time)
            local rtt = math.max(last_tried_time, self.config.retry_rtt_threshold)
            get_or_update_ewma(last_tried_endpoint, rtt, true, self.config.decay_time)
        end

        local filtered_peers
        for _, peer in ipairs(peers) do
            if not tried_endpoints[get_upstream_name(peer)] then
                if not filtered_peers then
                    filtered_peers = {}
                end
                table_insert(filtered_peers, peer)
            end
        end

        if not filtered_peers then
            ngx.log(ngx.WARN, "all endpoints have been retried")
            filtered_peers = util.deepcopy(peers)
        end

        if #filtered_peers > 1 then
            endpoint, ewma_score = pick_and_score(filtered_peers, k, self.config)
        else
            endpoint, ewma_score = filtered_peers[1], score(filtered_peers[1], self.config.decay_time)
        end
        local endpoint_name = get_upstream_name(endpoint)
        -- ewma_log("balance picked " .. endpoint_name .. " score " .. tostring(ewma_score))
        tried_endpoints[endpoint_name] = true
        ngx.ctx.balancer_ewma_tried_endpoint_last = endpoint_name
        ngx.ctx.balancer_ewma_tried_endpoint_time = ngx.now()
        -- ewma_log("retry update tried_endpoints " .. endpoint_name .. " now " .. tostring(now))
    end

    ngx.var.balancer_ewma_score = ewma_score

    return get_upstream_name(endpoint)
end

function _M.after_balance(self)
  -- only update ewma of the last endpoint
  local upstream = split.get_last_value(ngx.var.upstream_addr)
  if util.is_blank(upstream) then
    return
  end
  local response_time = tonumber(split.get_last_value(ngx.var.upstream_response_time)) or 0
  local connect_time = tonumber(split.get_last_value(ngx.var.upstream_connect_time)) or 0
  local rtt = connect_time + response_time
  get_or_update_ewma(upstream, rtt, true, self.config.decay_time)
end

---@param backend BackendGroup
function _M.sync(self, backend)
    if self.config_raw ~= backend.session_affinity_attribute then
        self.config = parse_ewma_config(backend)
        self.config_raw = backend.session_affinity_attribute
    end

    local normalized_endpoints_added, normalized_endpoints_removed =
        util.diff_endpoints(self.peers, backend.backends)
    if #normalized_endpoints_added == 0 and #normalized_endpoints_removed == 0 then
        -- ngx.log(ngx.INFO, "endpoints did not change for backend " .. tostring(backend.name))
        return
    end

    ngx_log(INFO, string_format("[%s] peers have changed for backend %s %s", self.name, backend.name, self.config_raw))

    self.peers = backend.backends
    self.peer_map = peer_map_helper.init_peer_map(backend)

    for _, endpoint_string in ipairs(normalized_endpoints_removed) do
        ngx.shared.balancer_ewma:delete(endpoint_string)
        ngx.shared.balancer_ewma_last_touched_at:delete(endpoint_string)
    end

    local slow_start_ewma = calculate_slow_start_ewma(self)
    if slow_start_ewma ~= nil then
        local now = ngx.now()
        for _, endpoint_string in ipairs(normalized_endpoints_added) do
            -- ewma_log("sync: added " .. endpoint_string .. " slow_start_ewma " .. tostring(slow_start_ewma) .. " now " ..
                -- tostring(now))
            store_stats(endpoint_string, slow_start_ewma, now)
        end
    end
end

---@param backend BackendGroup
function _M.new(self, backend)
    local o = {
        peers = backend.backends,
        peer_map = peer_map_helper.init_peer_map(backend),
        config_raw = backend.session_affinity_attribute,
        config = parse_ewma_config(backend),
    }
    setmetatable(o, self)
    self.__index = self
    return o
end

---comment
---@param self table
---@param node_key NodeKey
---@return BackendConf?
function _M.get_peer_conf(self, node_key)
    return peer_map_helper.get_peer_conf(self.peer_map, node_key)
end

return _M
