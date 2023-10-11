-- format:on
local _M = {}

local ngx_config = ngx.config

local current_subsystem = ngx_config.subsystem

local HTTP_SUBSYSTEM = "http"
local STREAM_SUBSYSTEM = "stream"

_M.CURRENT_SYBSYSTEM = current_subsystem
_M.HTTP_SUBSYSTEM = HTTP_SUBSYSTEM
_M.STREAM_SUBSYSTEM = STREAM_SUBSYSTEM

---@return boolean
function _M.is_http_subsystem()
    return current_subsystem == HTTP_SUBSYSTEM
end

---@return boolean
function _M.is_stream_subsystem()
    return current_subsystem == STREAM_SUBSYSTEM
end

return _M
