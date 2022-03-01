local _M = {}
function _M.curl(url, cfg)
    local httpc = require("resty.http").new()
    if cfg == nil then
        cfg = {}
        cfg["headers"] = {}
    end
    local res, err = httpc:request_uri(url, {method = "GET", headers = cfg.headers})
    return res, err
end
return _M
