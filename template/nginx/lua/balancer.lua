local common = require "utils.common"
local ngx_balancer = require "ngx.balancer"
local round_robin = require "balancer.round_robin"
local chash = require "balancer.chash"
local sticky_cookie = require "balancer.sticky_balanced"
local ngx = ngx
local ngx_log = ngx.log
local ngx_var = ngx.var
local ngx_exit = ngx.exit
local ngx_config = ngx.config
local ngx_shared = ngx.shared
local string_format = string.format

local _M = {}
local balancers = {}

local subsystem = ngx_config.subsystem
local DEFAULT_LB_ALG = "round_robin"
local IMPLEMENTATIONS = {
    round_robin = round_robin,
    chash = chash,
    sticky_cookie = sticky_cookie,
}

local function get_implementation(backend)
    local name = DEFAULT_LB_ALG
    if backend["session_affinity_policy"] ~= "" then
        if backend["session_affinity_policy"] == "sip-hash" then
            name = "chash"
        elseif backend["session_affinity_policy"] == "cookie" then
            name = "sticky_cookie"
        end
    end
    local implementation = IMPLEMENTATIONS[name]
    if not implementation then
        ngx_log(ngx.ERR, "failed to get implementation")
    end
    return implementation
end

local function format_ipv6_endpoints(endpoints)
    local formatted_endpoints = {}
    for _, endpoint in ipairs(endpoints) do
        local formatted_endpoint = endpoint
        if not endpoint.address:match("^%d+.%d+.%d+.%d+$") then
            formatted_endpoint.address = string.format("[%s]", endpoint.address)
        end
        table.insert(formatted_endpoints, formatted_endpoint)
    end
    return formatted_endpoints
end

local function sync_backend(backend)
    if not backend.backends or #backend.backends == 0 then
        ngx_log(ngx.INFO, string_format("there is no endpoint for backend %s. Removing...", backend.name))
        balancers[backend.name] = nil
        return
    end

    local implementation = get_implementation(backend)
    local balancer = balancers[backend.name]
    --{
    --  "mode": "http",
    --  "session_affinity_attribute": "",
    --  "name": "calico-new-yz-alb-09999-3a56db4e-20c3-42cb-82b8-fff848e8e6c3",
    --  "session_affinity_policy": "",
    --  "backends": [
    --    {
    --      "port": 80,
    --      "address": "10.16.12.9",
    --      "weight": 100
    --    }
    --  ]
    --}
    if not balancer then
        balancers[backend.name] = implementation:new(backend)
        return
    end

    -- every implementation is the metatable of its instances (see .new(...) functions)
    -- here we check if `balancer` is the instance of `implementation`
    -- if it is not then we deduce LB algorithm has changed for the backend
    if getmetatable(balancer) ~= implementation then
        ngx_log(ngx.INFO,
                string_format("LB algorithm changed from %s to %s, resetting the instance", balancer.name, implementation.name))
        balancers[backend.name] = implementation:new(backend)
        return
    end
    backend.backends = format_ipv6_endpoints(backend.backends)

    balancer:sync(backend)
end

function _M.sync_backends()
    local backends_data = ngx_shared[subsystem .. "_backend_cache"]:get("backend_group")
    if not backends_data then
        balancers = {}
        return
    end

    local new_backends, err = common.json_decode(backends_data)
    if not new_backends then
        ngx_log(ngx.ERR, "could not parse backends data: ", err)
        return
    end

    local balancers_to_keep = {}
    for _, new_backend in ipairs(new_backends) do
        sync_backend(new_backend)
        balancers_to_keep[new_backend.name] = balancers[new_backend.name]
    end

    for backend_name, _ in pairs(balancers) do
        if not balancers_to_keep[backend_name] then
            balancers[backend_name] = nil
        end
    end
end

local function get_balancer()
    local backend_name = ngx_var.upstream
    local balancer = balancers[backend_name]
    if not balancer then
        return
    end
    return balancer
end

function _M.balance()
    local balancer = get_balancer()
    if not balancer then
        ngx_log(ngx.ERR, "no balancer found for ", ngx_var.upstream)
        return
    end

    local peer = balancer:balance()
    if not peer then
        ngx.log(ngx.ERR, "no peer was returned, balancer: " .. balancer.name)
        return
    end

    ngx_balancer.set_more_tries(1)

    local ok, err = ngx_balancer.set_current_peer(peer)
    if not ok then
        ngx.log(ngx.ERR, string.format("error while setting current upstream peer %s: %s", peer, err))
    end

    -- TODO: dynamic keepalive connections pooling
    -- https://github.com/openresty/lua-nginx-module/pull/1600
    local _, err = ngx_balancer.set_timeouts(nil, nil, nil)
    if err ~= nil then
        ngx.log(ngx.ERR, err)
    end
end

return _M

