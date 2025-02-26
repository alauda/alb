-- format:on
--- Created by oilbeater.
--- DateTime: 17/8/17 上午10:35
---
--
local ipairs = ipairs
local next = next
local ngx = ngx
local ngx_log = ngx.log
local common = require "utils.common"
local dsl = require "match_engine.dsl"
local shm = require "config.shmap"
local cache = require "config.cache"

local _M = {}
local MY_POD_NAME = os.getenv("MY_POD_NAME") or ""

local function get_policies_of_port(key)
    ngx_log(ngx.INFO, string.format("cache: refresh policy cache from shm %s", key))
    local raw_policies = shm.get_port_rule(key)
    if raw_policies == nil then
        return nil, string.format("no %s", key)
    end
    local policies = common.json_decode(raw_policies)
    return policies
end

---comment
--- @param subsystem string http|stream
--- @param protocol  string tcp|udp
--- @param port  number
--- @return string|nil upstream
--- @return Policy|nil matched_policy
--- @return string|nil err_msg
function _M.get_upstream(subsystem, protocol, port)
    -- make sure rule_cahche are updated
    cache.rule_cache:update(0.1)
    local key = cache.gen_rule_key(subsystem, protocol, port)
    local policies, err, _ = cache.rule_cache:get(key, nil, get_policies_of_port, key)
    if err then
        ngx.log(ngx.ERR, "get policy from cache failed, " .. err)
        return nil, nil, tostring(err)
    end

    if policies == nil or next(policies) == nil then
        return nil, nil, "empty policy"
    end
    if subsystem == "http" then
        --[[ ngx.log(ngx.ERR, "try find a matched policy len ", #policies) ]]

        ngx.ctx.alb_ctx.var.trace.alb_pod = MY_POD_NAME
        for i, policy in ipairs(policies) do
            if (policy ~= nil and policy["dsl"] ~= nil) then
                local match, err = dsl.eval(policy["dsl"])
                --[[ ngx.log(ngx.ERR, "try find a matched policy ", policy["rule"]) ]]
                if (match) then
                    local trace = ngx.ctx.alb_ctx.var.trace
                    trace.rule = policy.rule
                    trace.index = tostring(i)
                    trace.upstream = policy.upstream
                    -- TODO add index in go
                    policy.index = i
                    --[[ ngx.log(ngx.ERR, "find a matched policy ", policy["rule"]) ]]
                    return policy.upstream, policy, nil
                end
                if (err ~= nil) then
                    ngx.log(ngx.ERR, "eval dsl " .. common.json_encode(policy["dsl"]) .. " failed " .. err)
                end
            end
        end
    elseif subsystem == "stream" and next(policies) ~= nil then
        return policies[1]["upstream"], policies[1], nil
    end

    -- return 404 if no rule match
    return nil, nil, "no rule match"
end

return _M
