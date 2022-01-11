-- a helper to get/set from ngx.shared
local ngx_config = ngx.config
local current_subsystem = ngx_config.subsystem
local ngx_shared = ngx.shared
local _M = {}

---get_raw
--- return the current subsystem raw policy string from ngx.shared.$current_subsystem+"_raw"
---@return string
function _M.get_raw()
    return ngx_shared[current_subsystem .. "_raw"]:get("raw")
end

---@param policy_raw string
function _M.set_raw(policy_raw)
    ngx_shared[current_subsystem .. "_raw"]:set("raw", policy_raw)
end

return _M