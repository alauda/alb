local os_getenv = os.getenv

local _M = {}
_M.default_ssl_strategy = os_getenv("DEFAULT-SSL-STRATEGY")
_M.default_https_port = 443

return _M
