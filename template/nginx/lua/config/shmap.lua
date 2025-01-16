local _M = {}

local ngx = ngx
local ngx_shared = ngx.shared

local subsys = require "utils.subsystem"
local current_subsystem = subsys.CURRENT_SYBSYSTEM

---@param key string
---@param value string|nil
function _M.set_stream_policy(key, value)
    ngx_shared["stream_policy"]:set(key, value)
end

---@param key string
function _M.del_stream_policy(key)
    ngx_shared["stream_policy"]:delete(key)
end

---@param key string
function _M.get_stream_policy(key)
    return ngx_shared["stream_policy"]:get(key)
end

---@param key string
function _M.del_http_cert(key)
    ngx_shared["http_certs_cache"]:delete(key)
end

---@param key string
---@param value string|nil
function _M.set_http_cert(key, value)
    ngx_shared["http_certs_cache"]:set(key, value)
end

---@param key string
function _M.get_http_cert(key)
    return ngx_shared["http_certs_cache"]:get(key)
end

---@param port string
function _M.del_http_rule(port)
    ngx_shared["http_policy"]:delete(port)
end

---@param port string
---@param value string|nil
function _M.set_http_rule(port, value)
    ngx_shared["http_policy"]:set(port, value)
end

---@param port string
function _M.get_http_rule(port)
    return ngx_shared["http_policy"]:get(port)
end

---@param port string
function _M.get_port_rule(port)
    if current_subsystem == subsys.HTTP_SUBSYSTEM then
        return _M.get_http_rule(port)
    end
    return _M.get_stream_policy(port)
end

local function config_key(key)
    return "/config/" .. key
end

---@param key string
function _M.del_config(key)
    ngx_shared["http_policy"]:delete(config_key(key))
end

---@param key string
---@param value string|nil
function _M.set_config(key, value)
    ngx_shared["http_policy"]:set(config_key(key), value)
end

---@param key string
function _M.get_config(key)
    return ngx_shared["http_policy"]:get(config_key(key))
end

function _M.get_policy_raw()
    return ngx_shared[current_subsystem .. "_raw"]:get("raw")
end

---@param value string|nil
function _M.set_policy_raw(value)
    ngx_shared[current_subsystem .. "_raw"]:set("raw", value)
end

-- TODO use mlchache
---@param value string|nil
function _M.set_backends(value)
    ngx_shared[current_subsystem .. "_backend_cache"]:set("backend_group", value)
end

--- @return string|nil
function _M.get_backends()
    return ngx_shared[current_subsystem .. "_backend_cache"]:get("backend_group") --[[@as string|nil]]
end

return _M
