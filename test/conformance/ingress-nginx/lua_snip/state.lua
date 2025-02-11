ngx.req.read_body()
ngx.log(ngx.INFO,
    "im state " .. ngx.var.http_id .. " " .. tostring(ngx.var.request_method) .. " " .. tostring(ngx.req.get_body_data()))
local id = ngx.var.http_id
local c = require("utils.common")

if ngx.shared.state:get(id) == nil then
    ngx.shared.state:set(id, c.json_encode({}, true))
end

if ngx.var.request_method == "PUT" then
    ngx.shared.state:set(id .. "-cfg", ngx.req.get_body_data())
    ngx.say("OK")
    return
end
if ngx.var.request_method == "GET" then
    local out = ngx.shared.state:get(id) or "{}"
    ngx.log(ngx.INFO, "state is " .. id .. " " .. tostring(out))
    ngx.header["Content-Type"] = "application/json"
    ngx.say(out)
end
