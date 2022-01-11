---
--- Created by oilbeater.
--- DateTime: 17/8/21 上午10:40
---
local ngx_shared = ngx.shared
local ngx_var = ngx.var
local ngx_header = ngx.header
local ngx_config = ngx.config

local method = ngx_var.request_method
local subsystem = ngx_config.subsystem

if (method == "GET") then
    ngx_header["content-type"] = "application/json"

    local policy = ngx_shared[subsystem .. "_raw"]:get("raw")
    ngx.print(policy)
else
    ngx.log(ngx.ERR, string.format("%s is not support", method))
    ngx.status = 501
    ngx.print("Method not support")
    return
end
