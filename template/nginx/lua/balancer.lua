-- format:on
-- THIS MODULE EVALED IN BOTH HTTP AND STREAM CTX
local common = require "utils.common"
local ngx_balancer = require "ngx.balancer"
local round_robin = require "balancer.round_robin"
local chash = require "balancer.chash"
local sticky = require "balancer.sticky_balanced"
local ngx = ngx
local ngx_log = ngx.log
local ngx_config = ngx.config
local ngx_shared = ngx.shared
local string_format = string.format
local ms2sec = common.ms2sec
local subsys = require "utils.subsystem"
local common = require "utils.common"
local e = require "error"

local _M = {}
local balancers = {}

local subsystem = ngx_config.subsystem
local DEFAULT_LB_ALG = "round_robin"
local IMPLEMENTATIONS = {round_robin = round_robin, chash = chash, sticky = sticky}

local function get_implementation(backend)
    local name = DEFAULT_LB_ALG
    if backend["session_affinity_policy"] ~= "" then
        if backend["session_affinity_policy"] == "sip-hash" then
            name = "chash"
        elseif backend["session_affinity_policy"] == "cookie" then
            name = "sticky"
        elseif backend["session_affinity_policy"] == "header" then
            name = "sticky"
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
        balancers[backend.name] = nil
        return
    end

    local implementation = get_implementation(backend)
    local balancer = balancers[backend.name]
    if not balancer then
        balancers[backend.name] = implementation:new(backend)
        return
    end

    -- every implementation is the metatable of its instances (see .new(...) functions)
    -- here we check if `balancer` is the instance of `implementation`
    -- if it is not then we deduce LB algorithm has changed for the backend
    if getmetatable(balancer) ~= implementation then
        ngx_log(ngx.INFO, string_format("LB algorithm changed from %s to %s, resetting the instance", balancer.name, implementation.name))
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
    local backend_name = ngx.ctx.upstream
    local balancer = balancers[backend_name]
    if not balancer then
        return
    end
    return balancer
end

--- get the policy which match this request.
local function get_policy()
    local policy = ngx.ctx.matched_policy
    if policy == nil then
        ngx_log(ngx.ERR, "impossible! no policy find for req on balance")
    end
    return policy
end

function _M.balance()
    local balancer = get_balancer()
    local policy = get_policy()
    if not balancer then
        local msg = "no balancer found for " .. ngx.ctx.upstream
        ngx_log(ngx.ERR, msg)
        e.exit(e.InvalidBalancer, msg)
        return
    end

    local peer = balancer:balance()
    if not peer then
        ngx.log(ngx.ERR, "no peer was returned, balancer: " .. balancer.name)
        e.exit(e.InvalidBalancer, "no peer")
        return
    end
    -- TODO 在实现retrypolicy时这里需要被重写。注意测试。
    ngx_balancer.set_more_tries(1)

    local ok, err = ngx_balancer.set_current_peer(peer)
    if not ok then
        ngx.log(ngx.ERR, string.format("error while setting current upstream peer %s: %s", peer, err))
        e.exit(e.InvalidBalancer, "set peer fail")
    end

    -- TODO: dynamic keepalive connections pooling
    -- https://github.com/openresty/lua-nginx-module/pull/1600
    -- ngx.log(ngx.NOTICE, "send timeout "..common.json_encode(policy))

    if common.has_key(policy, {"config", "timeout"}) then
        local timeout = policy.config.timeout
        local proxy_connect_timeout_secs = ms2sec(timeout.proxy_connect_timeout_ms)
        local proxy_send_timeout_secs = ms2sec(timeout.proxy_send_timeout_ms)
        local proxy_read_timeout_secs = ms2sec(timeout.proxy_read_timeout_ms)
        -- ngx.log(ngx.NOTICE,
        -- string.format("set timeout rule %s pconnect %s psend %s pread %s\n", policy.rule,
        --     tostring(proxy_connect_timeout_secs), tostring(proxy_send_timeout_secs), tostring(proxy_read_timeout_secs)))
        local _, err = ngx_balancer.set_timeouts(proxy_connect_timeout_secs, proxy_send_timeout_secs, proxy_read_timeout_secs)
        if err ~= nil then
            ngx.log(ngx.ERR, err)
            e.exit(e.InvalidBalancer, "set timeout fail")
        end
    end

    -- set balancer is the last step of send a request
    if subsys.is_http_subsystem() and ngx.ctx.alb_ctx["http_cpaas_trace"] == "true" then
        ngx.ctx.alb_ctx.trace.upstream_ip = peer
        -- nginx will merge same response header into one.
        -- if upstram is alb either, it will set response's header x-cpaas-trace too.
        -- since that, at the first alb, the response' header will be a list of all trace info.
        ngx.header["x-cpaas-trace"] = common.json_encode(ngx.ctx.alb_ctx.trace, false)
    end
end

return _M
