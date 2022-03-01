---
--- Created by oilbeater.
--- DateTime: 17/8/17 上午10:35
---
--

local ipairs = ipairs
local next = next
local ngx = ngx
local ngx_log = ngx.log
local ngx_shared = ngx.shared
local ngx_config = ngx.config
local common = require "utils.common"
local dsl = require "dsl"
local cache = require "cache"

local _M = {}

local SUBSYSTEM = ngx_config.subsystem

local function get_policies(key)
    local shared_key = SUBSYSTEM .. "_policy"
    ngx_log(ngx.INFO, string.format("refresh cache from ngx.share[%s][%s]", shared_key, key))
    local raw_policies = ngx_shared[shared_key]:get(key)
    if raw_policies == nil then
        return nil, string.format("no policies found on ngx.share[%s][%s] ", shared_key, key)
    end
    local policies = common.json_decode(raw_policies)
    return policies
end

-- @param: subsystem string http|stream
-- @param: protocol  tcp|udp
-- @param: port  number
-- @ret: upstream string
-- @ret: matched_policy table
-- @ret: err_msg string
function _M.get_upstream(subsystem, protocol, port)
    local upstream = "default"
    cache.rule_cache:update(0.1)
    local key = cache.gen_rule_key(subsystem, protocol, port)
    local policies, err = cache.rule_cache:get(key, nil, get_policies, key)
    if err then
        ngx.log(ngx.ERR, "get policy from cache failed, " .. err)
    end

    local errmsg = "unknown error"
    if (policies) then
        if subsystem == "http" then
            for _, policy in ipairs(policies) do
                if (policy ~= nil and policy["dsl"] ~= nil) then
                    local match, err = dsl.eval(policy["dsl"])
                    if (match) then
                        -- ngx.log(ngx.ERR,"find a matched policy ",policy["rule"])
                        return policy["upstream"], policy, nil
                    end

                    if (err ~= nil) then
                        ngx.log(ngx.ERR, "eval dsl " .. common.json_encode(policy["dsl"]) .. " failed " .. err)
                    end
                end
            end
        elseif subsystem == "stream" then
            if next(policies) ~= nil then
                return policies[1]["upstream"], policies[1], nil
            end
        end

        -- return 404 if no rule match
        if (upstream == "default") then
            errmsg = "Resource not found, no rule match"
        end
    else
        -- no policies on this port
        errmsg = "Resource not found, no policies on this port:" .. port
    end

    return nil, nil, errmsg
end

return _M
