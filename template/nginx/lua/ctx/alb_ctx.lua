--- per request ctx

local _M = {}
local str_sub = string.sub
local var_proxy = require "ctx.var_proxy"

---@class BackendConf
---@field address string
---@field port number
---@field weight number
---@field svc string?
---@field ns string?

---@class PeerConf
---@field peer string
---@field conf BackendConf   the policy.backend_groups[upstream].backends[current_backend_index]


---@class AlbCtx
---@field matched_policy Policy
---@field send_count number  retry_count
---@field peer PeerConf      valid only after the balance phase
---@field var table          var proxy
---@field otel OtelCtx?
---@field auth AuthCtx?

---@param ctx AlbCtx
function _M.get_last_upstream_status(ctx)
    -- $upstream_status maybe including multiple status, only need the last one
    return tonumber(str_sub(ctx.var["upstream_status"] or "", -3))
end

---@return AlbCtx
function _M.new()
    return {
        var = var_proxy.new(),
        send_count = 0,
        matched_policy = nil,
    }
end

---@return AlbCtx
function _M.get_alb_ctx()
    return ngx.ctx.alb_ctx
end

return _M
