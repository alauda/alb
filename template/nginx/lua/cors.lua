local string_sub = string.sub
local string_find = string.find
local string_len = string.len

local _M = {}

---comment
---@param allow_origin string
---@param origin string
---@return boolean
function _M.origin_contains(allow_origin, origin)
    local sindex, eindex = string_find(allow_origin, origin, 1, true)
    if sindex == nil then
        return false
    end
    local allow_origin_len = string_len(allow_origin)
    -- the last index in allow_origin either be last or a ,
    return eindex == allow_origin_len or string_sub(allow_origin, eindex + 1, eindex + 1) == ","
end

function _M.init_allow_origin(matched_policy, ngx_header)
    local allow_origin = matched_policy["cors_allow_origin"]
    if allow_origin == "" then
        ngx_header['Access-Control-Allow-Origin'] = "*"
        return
    end

    local user_origin = ngx.ctx.alb_ctx.var["http_origin"] or ""
    if user_origin == "" then
        ngx_header['Access-Control-Allow-Origin'] = allow_origin
        return
    end
    if _M.origin_contains(allow_origin, user_origin) then
        ngx_header['Access-Control-Allow-Origin'] = user_origin
    end
end

function _M.common_cors_header(matched_policy, ngx_header)
    _M.init_allow_origin(matched_policy, ngx_header)

    ngx_header['Access-Control-Allow-Credentials'] = 'true'
    ngx_header['Access-Control-Allow-Methods'] = 'GET, PUT, POST, DELETE, PATCH, OPTIONS'
    if matched_policy["cors_allow_headers"] ~= "" then
        ngx_header['Access-Control-Allow-Headers'] = matched_policy["cors_allow_headers"]
    else
        ngx_header['Access-Control-Allow-Headers'] =
        'DNT,X-CustomHeader,Keep-Alive,User-Agent,X-Requested-With,If-Modified-Since,Cache-Control,Content-Type,Authorization'
    end
end

function _M.header_filter()
    local matched_policy = ngx.ctx.matched_policy
    local enable_cors = matched_policy["enable_cors"]
    if not enable_cors then
        return
    end
    if ngx.ctx.alb_ctx.var.method ~= 'OPTIONS' then
        _M.common_cors_header(matched_policy, ngx.header)
    end
end

return _M
