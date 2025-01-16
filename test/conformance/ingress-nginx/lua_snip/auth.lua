local c = require("utils.common")
ngx.log(ngx.INFO, "im auth")
local id = ngx.var.http_id
local h, err = ngx.req.get_headers()
if err ~= nil then
    ngx.log(ngx.ERR, "err: " .. tostring(err))
end

if ngx.shared.state:get(id) == nil then
    local data = c.json_encode({}, true)
    ngx.log(ngx.ERR, "init state ", data, id)
    ngx.shared.state:set(id, data)
end

ngx.log(ngx.ERR, "state is " .. id .. " " .. tostring(ngx.shared.state:get(id)))
local data = c.json_decode(ngx.shared.state:get(id))
data["/auth"] = h
ngx.shared.state:set(id, c.json_encode(data))

for k, v in pairs(h) do
    ngx.log(ngx.ERR, "auth " .. tostring(k) .. " : " .. tostring(v))
end

local cfg = c.json_decode(ngx.shared.state:get(id .. "-cfg"))
for k, v in pairs(cfg.auth_response_header) do
    ngx.header[k] = v
end

ngx.log(ngx.ERR, "auth exit with " .. tostring(cfg.auth_exit))
ngx.status = cfg.auth_exit
ngx.exit(cfg.auth_exit)
ngx.say(cfg.auth_response_body)
