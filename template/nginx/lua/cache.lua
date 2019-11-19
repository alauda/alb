local mlcache = require "resty.mlcache"

local _M = {}

function _M.init_mlcache(name, shared_dict, opt)
    local c, err = mlcache.new(name, shared_dict, opt)
    if not c then
        ngx.log(ngx.ERR, "create mlcache failed, " .. err)
    end
    _M[name] = c
end

return _M