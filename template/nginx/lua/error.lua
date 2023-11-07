-- format:on
local _M = {}
local ngx = ngx

local ErrReason = "X-ALB-ERR-REASON"

_M.UnknowStickyPolicy = "UnknowStickyPolicy"
_M.InvalidUpstream = "InvalidUpstream"
_M.InvalidBalancer = "InvalidBalancer"
_M.BackendError = "BackendError"
_M.TimeoutViaAlb = "TimeoutViaAlb"
_M.TimeoutViaBackend = "TimeoutViaBackend"

---comment
-- exit with code 500 Internal Server Error
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
    ngx.ctx.is_alb_err = true
    ngx.exit(code)
end

function _M.timeout_via_alb()
    ngx.header[ErrReason] = _M.TimeoutViaAlb
end

function _M.timeout_via_backend()
    ngx.header[ErrReason] = _M.TimeoutViaBackend
end

function _M.backend_error(_, msg)
    ngx.header[ErrReason] = _M.BackendError .. " : " .. tostring(msg)
end

return _M
