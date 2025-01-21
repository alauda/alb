local c = require("utils.common")
if ngx.var.uri == "/ok" then
    ngx.say("ok")
    return
end
local id = ngx.var.http_id
if id == nil then
    ngx.say("ok")
    return
end
ngx.log(ngx.INFO, "im app " .. id)
local h, err = ngx.req.get_headers()
if err ~= nil then
    ngx.log(ngx.ERR, "err: " .. tostring(err))
end
for k, v in pairs(h) do
    ngx.log(ngx.ERR, "app " .. tostring(k) .. " : " .. tostring(v))
end
if ngx.shared.state:get(id) == nil then
    ngx.shared.state:set(id, c.json_encode({}))
end

local data = c.json_decode(ngx.shared.state:get(id))
data["/"] = h
ngx.shared.state:set(id, c.json_encode(data))

local data = c.json_decode(ngx.shared.state:get(id .. "-cfg"))
for k, v in pairs(data.app_response_header) do
    ngx.header[k] = v
end
ngx.status = data.app_exit
ngx.say(data.app_response_body)
ngx.exit(data.app_exit)
ngx.say("OK")
