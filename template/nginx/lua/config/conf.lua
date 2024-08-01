-- format:on
local os_getenv = os.getenv

local _M = {}

_M.default_ssl_strategy = os_getenv("DEFAULT_SSL_STRATEGY")
_M.default_https_port = os_getenv("INGRESS_HTTPS_PORT")

return _M
