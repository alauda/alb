---
--- Created by oilbeater.
--- DateTime: 17/8/21 上午10:40
---

local common = require "common"
local dsl   = require "dsl"

local method = ngx.var.request_method

if(method == "GET") then
    ngx.header["content-type"] = "application/json"
    local policies = ngx.shared.http_policy:get("all_policies") or "{}"
    local certs = ngx.shared.certs_cache:get("certificate_map") or "{}"
    ngx.print('{"port_map" :'.. policies ..', "certificate_map": ' .. certs .. '}')
elseif(method == "PUT") then
    ngx.req.read_body()
    ngx.header["content-type"] = "application/json"
    local data = ngx.req.get_body_data()
    if(data == nil) then
        ngx.status = 400
        ngx.print("no request body")
        return
    end
    local dict_data = common.json_decode(data)
    local all_ports_policies = dict_data["port_map"]
    local certificate_map = dict_data["certificate_map"]
    ngx.shared.http_policy:set("all_policies", common.json_encode(all_ports_policies))
    ngx.shared.certs_cache:set("certificate_map", common.json_encode(certificate_map, true))

    --split policies by port to decrease json operation overhead
    --parse raw dsl to ast to decrease overhead
    for port, policies in pairs(all_ports_policies)
    do
        for _, policy in ipairs(policies)
        do
            if(policy ~= nil and policy["rule"] ~= nil) then
                policy["rule"] = dsl.generate_ast(policy["rule"])

            end
        end
        ngx.shared.http_policy:set(port, common.json_encode(policies))
    end
    ngx.print(data)
    return

else
    ngx.log(ngx.ERR, string.format("%s is not support", method))
    ngx.status = 501
    ngx.print("Method not support")
    return
end
