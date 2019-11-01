if ngx.ctx.matched_policy ~= nil then
  local rewrite_target = ngx.ctx.matched_policy["rewrite_target"]
  local policy_url = ngx.ctx.matched_policy["url"]
  if rewrite_target ~= "" then
    if policy_url == "" then
      policy_url = "/"
    end
    new_uri, _, _ = ngx.re.sub(ngx.var.uri, policy_url, rewrite_target, "jo")
    ngx.req.set_uri(new_uri, false)
  end
else
  ngx.status = 404
  ngx.say(ngx.ctx.errmsg)
end
