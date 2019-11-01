---
--- Created by oilbeater.
--- DateTime: 17/8/17 上午10:35
---
--

local common = require "common"
local dsl = require "dsl"

local policies = ngx.shared.http_policy:get(ngx.var.server_port)
local upstream = "default"

if(policies) then
    policies = common.json_decode(policies)
    for _, policy in ipairs(policies) do
        if(policy ~= nil and policy["rule"] ~= nil) then
            local match, err = dsl.eval(policy["rule"])
            if(match) then
                upstream = policy["upstream"]
                ngx.ctx.matched_policy = policy
                return upstream
            end

            if(err ~= nil ) then
                ngx.log(ngx.ERR, "eval dsl %s failed %s", common.json_encode(policy["rule"]), err)
            end
        end
    end

    -- return 404 if no rule match
    if(upstream == "default") then
        ngx.ctx.errmsg = "Resource not found, no rule match"
    end
else
    -- no policies on this port
    ngx.ctx.errmsg = "Resource not found, no policies on this port"
end

ngx.log(ngx.ERR, "cant find upstream for req: " .. ngx.var.scheme.. "://" .. ngx.var.http_host .. ngx.var.request_uri)

return upstream
