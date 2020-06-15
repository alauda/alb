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
local common = require "common"
local dsl = require "dsl"
local cache = require "cache"

local _M = {}

local subsystem = ngx_config.subsystem

local function get_port_policies(port)
    ngx_log(ngx.INFO, "refresh cache for port:", port)
    local raw_policies = ngx_shared[subsystem .. "_policy"]:get(port)
    if raw_policies == nil then
        return nil, "no policies found on this port:" .. port
    end
    local policies = common.json_decode(raw_policies)
    return policies
end

-- @param: port
-- @ret: upstream
-- @ret: matched_policy
-- @ret: errmsg
function _M.get_upstream(port)
    local upstream = "default"
    cache.rule_cache:update()
    local policies, err = cache.rule_cache:get(port, nil, get_port_policies, port)
    if err then
        ngx.log(ngx.ERR, "get mlcache failed, " .. err)
    end

    local errmsg = "unknown error"
    if(policies) then
        if subsystem == "http" then
            do
                --[
                --  {
                --    "priority": 100,
                --    "rule": rule_name,
                --    "upstream": "calico-new-yz-alb-09999-3a56db4e-20c3-42cb-82b8-fff848e8e6c3",
                --    "protocol": "http",
                --    "url": "/s1",
                --    "dsl": [
                --      "AND",
                --      [
                --        "STARTS_WITH",
                --        "URL",
                --        "/s1"
                --      ]
                --    ],
                --    "rewrite_target": "/server_addr"
                --  }
                --]
            end
            for _, policy in ipairs(policies) do
                if(policy ~= nil and policy["dsl"] ~= nil) then
                    local match, err = dsl.eval(policy["dsl"])
                    if(match) then
                        return policy["upstream"], policy, nil
                    end

                    if(err ~= nil ) then
                        ngx.log(ngx.ERR, "eval dsl " .. common.json_encode(policy["dsl"]) .. " failed " .. err)
                    end
                end
            end
        elseif subsystem == "stream" then
            do
                --[
                --  {
                --    "priority": 0,
                --    "upstream": "calico-new-yz-alb-9997-tcp",
                --    "protocol": "tcp",
                --    "url": "",
                --    "rewrite_target": ""
                --  }
                --]
            end
            if next(policies) ~= nil then
               return policies[1]["upstream"], policies[1], nil
            end
        end

        -- return 404 if no rule match
        if(upstream == "default") then
            errmsg = "Resource not found, no rule match"
        end
    else
        -- no policies on this port
        errmsg = "Resource not found, no policies on this port:" .. port
    end

    return nil, nil, errmsg
end

return _M
