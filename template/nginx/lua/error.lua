-- format:on
local _M = {}
local ngx = ngx
local common = require "utils.common"
local subsys = require "utils.subsystem"

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
    if subsys.is_http_subsystem() then
        ngx.header[ErrReason] = reason
        ngx.ctx.is_alb_err = true
        ngx.status = code
        if ngx.ctx.alb_ctx.var["http_cpaas_trace"] == "true" then
            ngx.header["x-cpaas-trace"] = common.json_encode(ngx.ctx.alb_ctx.var.trace, false)
        end
        ngx.exit(ngx.HTTP_OK)
    end
    if subsys.is_stream_subsystem() then
        ngx.exit(ngx.ERROR)
    end
end

function _M.http_backend_error(_, msg)
    ngx.header[ErrReason] = _M.BackendError .. " : " .. tostring(msg)
end

return _M
