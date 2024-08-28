-- format:on
--- DateTime: 2021/11/16 下午6:27
local common = require "utils.common"
local ngx_resp = require "ngx.resp"
local ngx_req = require "ngx.req"
local string_sub = string.sub
local string_len = string.len

local _M = {}

function _M.rewrite_response_header()
    local matched_policy = ngx.ctx.matched_policy
    if not common.has_key(matched_policy, {"config", "rewrite_response"}) then
        return
    end
    local cfg = matched_policy["config"]["rewrite_response"]
    local ngx_header = ngx.header
    local headers_set = cfg["headers"]
    if headers_set then
        for k, v in pairs(headers_set) do
            ngx_header[k] = v
        end
    end

    local headers_remove = cfg["headers_remove"]
    if headers_remove then
        for _, h in pairs(headers_remove) do
            if ngx.header[h] then
                ngx_header[h] = nil
            end
        end
    end

    local headers_add = cfg["headers_add"]
    if headers_add then
        for k, vlist in pairs(headers_add) do
            ngx_resp.add_header(k, vlist)
        end
    end

end

---comment
---@param key string
---@param val string|nil
local function trim_cookie_quote_if_needed(key, val)
    if val == nil then
        return nil
    end
    if string_sub(key, 1, 7) ~= "cookie_" then
        return val
    end
    if string_len(val) <= 1 then
        return val
    end
    if string_sub(val, 1, 1) == '"' and string_sub(val, -1, -1) == '"' then
        return string_sub(val, 2, -2)
    end
    return val
end

function _M.rewrite_request_header()
    local matched_policy = ngx.ctx.matched_policy
    if not common.has_key(matched_policy, {"config", "rewrite_request"}) then
        return
    end
    local cfg = matched_policy["config"]["rewrite_request"]
    local headers_set = cfg["headers"]
    if headers_set then
        for k, v in pairs(headers_set) do
            ngx.req.set_header(k, v)
        end
    end
    local headers_set_var = cfg["headers_var"]
    if headers_set_var then
        for k, varname in pairs(headers_set_var) do
            local var = ngx.ctx.alb_ctx.var[varname]
            var = trim_cookie_quote_if_needed(varname, var)
            if var then
                ngx.req.set_header(k, var)
            end
        end
    end

    local headers_remove = cfg["headers_remove"]
    if headers_remove then
        for _, h in pairs(headers_remove) do
            ngx.req.clear_header(h)
        end
    end

    local headers_add = cfg["headers_add"]
    if headers_add then
        for k, vlist in pairs(headers_add) do
            ngx_req.add_header(k, vlist)
        end
    end

    local headers_add_var = cfg["headers_add_var"]
    if headers_add_var then
        for k, varlist in pairs(headers_add_var) do
            for _, varname in pairs(varlist) do
                local var = ngx.ctx.alb_ctx.var[varname]
                var = trim_cookie_quote_if_needed(varname, var)
                if var then
                    ngx_req.add_header(k, var)
                end
            end
        end
    end

end

return _M
