---
--- Created by oilbeater.
--- DateTime: 17/8/21 上午10:40
---

local string_lower = string.lower
local ngx_shared = ngx.shared
local ngx_var = ngx.var
local ngx_req = ngx.req
local ngx_header = ngx.header
local ngx_config = ngx.config

local method = ngx_var.request_method
local subsystem = ngx_config.subsystem

if(method == "GET") then
    ngx_header["content-type"] = "application/json"
    local args = ngx_req.get_uri_args()

    local policies = ngx_shared[subsystem .. "_policy"]:get("all_policies") or "{}"
    local certs = ngx_shared[subsystem .. "_certs_cache"]:get("certificate_map") or "{}"
    local backends = ngx_shared[subsystem .. "_backend_cache"]:get("backend_group") or "{}"

    local q = args["q"]
    local p = args["p"]
    if q then
        if q == "policies" then
            if p then
                ngx.print(ngx_shared[subsystem .. "_policy"]:get(p))
            else
                ngx.print(policies)
            end
        elseif q == "certs" then
            if p then
                ngx.print(ngx_shared[subsystem .. "_certs_cache"]:get(string_lower(p)))
            else
                ngx.print(certs)
            end
        elseif q == "backends" then
            if p then
                ngx.print(ngx_shared[subsystem .. "_backend_cache"]:get(p))
            else
                ngx.print(backends)
            end
        end
    else
        ngx.print('{"port_map": '.. policies .. ', "backend_group": ' .. backends .. ', "certificate_map": ' .. certs ..  '}')
    end
else
    ngx.log(ngx.ERR, string.format("%s is not support", method))
    ngx.status = 501
    ngx.print("Method not support")
    return
end
