local _m = {}
local cache = require("config.cache")
local json = require "cjson"
local eh = require("error")
local ngx_balancer = require "ngx.balancer"

--- milliseconds to seconds,if ms is nil return nil
--- @param ms number|nil
---@return number|nil
local function ms2sec(ms)
    if ms == nil or ms == json.null then
        return nil
    end
    if ms <= 0 then
        return nil
    end
    return ms / 1000
end

---@param ctx AlbCtx
function _m.balancer_hook(ctx)
    local timeout_cfg, err = _m.get_config(ctx)
    if err ~= nil or timeout_cfg == nil then
        return
    end

    local connect = ms2sec(timeout_cfg.proxy_connect_timeout_ms)
    local send = ms2sec(timeout_cfg.proxy_send_timeout_ms)
    local read = ms2sec(timeout_cfg.proxy_read_timeout_ms)
    -- ngx.log(ngx.ERR, "[debug] set timeout ", connect, " ", send, " ", read)
    local _, err = ngx_balancer.set_timeouts(connect, send, read)
    if err ~= nil then
        eh.exit("set timeout fail", err)
    end
end

---@param ctx AlbCtx
---@return TimeoutCr?
---@return any? error
function _m.get_config(ctx)
    return cache.get_config_from_policy(ctx.matched_policy, "timeout")
end

return _m
