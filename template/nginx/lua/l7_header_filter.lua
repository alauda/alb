local ngx = ngx
local matched_policy = ngx.ctx.matched_policy
if matched_policy == nil then
    return
end
local cors = require "cors"
local rewrite_header = require "rewrite_header"

cors.header_filter()
rewrite_header.rewrite_response_header()

