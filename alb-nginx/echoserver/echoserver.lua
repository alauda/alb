---
--- Created by oilbeater.
--- DateTime: 17/11/1 下午12:18
---

ngx.req.read_body()

local cjson = require "cjson"
local payload = {
    hostname = os.getenv("HOSTNAME"),
    version  = os.getenv("VERSION"),
    uri      = ngx.var.uri,
    param    = ngx.req.get_uri_args(),
    data     = ngx.req.get_body_data(),
    headers  = ngx.req.get_headers()
}

ngx.header["content-type"] = "application/json"
ngx.say(cjson.encode(payload))
