local mlcache = require "resty.mlcache"

local _M = {}

function _M.init_mlcache(name, shared_dict, opt)
    local c, err = mlcache.new(name, shared_dict, opt)
    if not c then
        ngx.log(ngx.ERR, "create mlcache failed, " .. err)
    end
    _M[name] = c
end

---gen_rule_key
---@param subsystem "stream"|"http"
---@param protocol  "tcp"|"udp"
---@param port number
---@return string
function _M.gen_rule_key(subsystem, protocol, port)
    return string.format("%s_%s_%d", subsystem, string.lower(protocol), port)
end

return _M