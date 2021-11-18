local ngx = ngx
local matched_policy = ngx.ctx.matched_policy
if matched_policy == nil then
    return
end
local cors = require "cors"
local rewrite_response = require "rewrite_response"

cors.header_filter()
rewrite_response.header_filter()

