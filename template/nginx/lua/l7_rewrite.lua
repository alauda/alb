local ngx_var = ngx.var
local ngx_re = ngx.re
local ngx_req = ngx.req
local ngx_log = ngx.log
local ngx_say = ngx.say
local ngx_exit = ngx.exit
local upstream = require "upstream"

local t_upstream, matched_policy, errmsg = upstream.get_upstream(ngx_var.server_port)
if t_upstream ~= nil then
  ngx_var.upstream = t_upstream
end
if matched_policy ~= nil then
  local rewrite_target = matched_policy["rewrite_target"]
  local policy_url = matched_policy["url"]
  if rewrite_target ~= "" then
    if policy_url == "" then
      policy_url = "/"
    end
    local new_uri = ngx_re.sub(ngx_var.uri, policy_url, rewrite_target, "jo")
    ngx_req.set_uri(new_uri, false)
  end
elseif errmsg ~= nil then
  ngx.status = 404
  ngx_log(ngx.ERR, errmsg)
  ngx_say(errmsg)
  ngx_exit(ngx.HTTP_OK)
end
