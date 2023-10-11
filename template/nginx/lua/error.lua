-- format:on
local _M = {}
local ngx = ngx

local ErrReason = "X-ALB-ERR-REASON"

_M.UnknowStickyPolicy = "UnknowStickyPolicy"
_M.InvalidUpstream = "InvalidUpstream"

---comment
---@param reason string
---@param msg? string
function _M.exit(reason, msg)
    _M.exit_with_code(reason, msg, ngx.ERROR)
end

function _M.exit_with_code(reason, msg, code)
    if msg ~= nil then
        reason = reason .. " : " .. tostring(msg)
    end
    ngx.header[ErrReason] = reason
    ngx.exit(code)
end
return _M
