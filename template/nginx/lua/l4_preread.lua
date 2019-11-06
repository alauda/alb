local ngx_var = ngx.var
local ngx_log = ngx.log
local ngx_say = ngx.say
local ngx_exit = ngx.exit

local upstream = require "upstream"

local t_upstream, _, errmsg = upstream.get_upstream(ngx_var.server_port)
if t_upstream ~= nil then
  ngx_var.upstream = t_upstream
end

if errmsg ~= nil then
  ngx.status = 404
  ngx_log(ngx.ERR, errmsg)
  ngx_say(errmsg)
  ngx_exit(200)
end
