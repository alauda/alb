local os_getenv = os.getenv

local _M = {}
local function _get_default_conf()
    local new = os_getenv("DEFAULT_SSL_STRATEGY")
    if new ~= nil then
        ngx.log(ngx.ERR,"xx "..new)
        return new
    end

    return os_getenv("DEFAULT_SSL_STRATEGY")
end
_M.default_ssl_strategy = _get_default_conf() 
_M.default_https_port = os_getenv("INGRESS_HTTPS_PORT")

return _M