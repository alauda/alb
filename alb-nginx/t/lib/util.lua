local _M = {}

local inspect = require"inspect"

function _M.curl(url, cfg)
    local httpc = require("resty.http").new()
    if cfg == nil then
        cfg = {}
        cfg["headers"] = {}
    end
    local res, err = httpc:request_uri(url, {method = "GET", headers = cfg.headers})
    return res, err
end

_M.inspect = inspect

function _M.log(msg)
	ngx.log(ngx.NOTICE, "alb_debug:"..msg.."alb_debug_end")
end

function _M.now_ms()
	ngx.update_time()
	return ngx.now()*1000
end

function _M.time_spend(f)
	local start = _M.now_ms()
	_M.log("time spend start "..tostring(ngx.now()))
	local ret = {f()}
	local stop =  _M.now_ms()
	_M.log("time spend end "..tostring(ngx.now()))
	return stop-start,unpack(ret)
end

return _M
