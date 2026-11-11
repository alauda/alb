local ngx = ngx
local stash_ctx = require "ctx.stash_ctx"
local rewrite_header = require "rewrite_header"

local function error_page_phase()
    stash_ctx.apply_ref()
    local matched_policy = ngx.ctx.matched_policy
    if matched_policy ~=nil then
        -- only rewrite_response need to be applied on error page
        rewrite_header.rewrite_response_header(matched_policy)
    end
    ngx.say("X-Error: " .. ngx.status)
    ngx.exit(ngx.OK)
end

return error_page_phase()