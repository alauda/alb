local ngx = ngx
local ngx_config = ngx.config
local subsystem = ngx_config.subsystem
local re_gsub = ngx.re.gsub
local resty_var = require("resty.ngxvar")
local common = require "utils.common"

local _M = {}

local ngxvar_vars = {
    uri=true, host=true, remote_addr=true, scheme=true
}
local param_prefix = "param_"
local header_prefix = "header_"
local cookie_prefix = "cookie_"

local mt = {
    __index = function(t, key)
        if key == "method" then
            return t._method
        end
        if ngxvar_vars[key] then
            if t._var[key] then
                return t._var[key]
            end
            local val = resty_var.fetch(key, t._req)
            t._var[key] = val
            return val
        end
        if common.has_prefix(key, header_prefix) then
            local trimed = common.trim(key, header_prefix)
            if t._header[trimed] then
                return t._header[trimed]
            end
            key = key:lower()
            key = re_gsub(key, "-", "_", "jo")
            local val = resty_var.fetch(key, t._req)
            t._header[trimed] = val
            return val
        end
        if common.has_prefix(key, cookie_prefix) then
            local trimed = common.trim(key, cookie_prefix)
            if t._cookie[trimed] then
                return t._cookie[trimed]
            end
            local val = resty_var.fetch(key, t._req)
            t._cookie[trimed] = val
            return val
        end
        if common.has_prefix(key, param_prefix) then
            local trimed = common.trim(key, param_prefix)
            if t._args[trimed] then
                return t._args[trimed]
            end
            local args = ngx.req.get_uri_args()
            t._args = args
            return t._args[trimed]
        end
        if t._var[key] then
            return t._var[key]
        else
            local val = resty_var.fetch(key, t._req)
            t._var[key] = val
            return val
        end
    end
}

function _M.new(self)
    local req = resty_var.request()
    local method
    if subsystem == "http" then
        method = ngx.req.get_method()
    end
    local var = {}
    local cookie = {}
    local header = {}
    local args = {}
    return setmetatable({_req = req, _method = method, _var = var,
                                _cookie = cookie, _header = header, _args = args}, mt)
end

return _M