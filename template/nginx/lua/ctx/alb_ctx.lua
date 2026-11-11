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
---@field peer NodeKey
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

---@param ctx AlbCtx
---@param ngx_var string
---@return any?
function _M.get_ngx_var(ctx, ngx_var)
    return ctx.var[ngx_var]
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

---@return string
---@return string
function _M.get_peer_svc_ns_name()
    local ctx = _M.get_alb_ctx()
    if not ctx then
        return "", ""
    end
    if not ctx.peer or not ctx.peer.conf or type(ctx.peer.conf) ~= "table" then
        return "", ""
    end
    return ctx.peer.conf.ns or "", ctx.peer.conf.svc or ""
end

function _M.gen_rule_id()
    local ctx = _M.get_alb_ctx()
    if not ctx then
        return ""
    end
    if not ctx.matched_policy then
        return ""
    end
    local source = ctx.matched_policy.source
    if not source or not source.source_name or source.source_name == "" then
        return ctx.matched_policy.rule
    end
    local ns = source.source_ns or ""
    local name = source.source_name or ""
    return ns .. "/" .. name
end

return _M
