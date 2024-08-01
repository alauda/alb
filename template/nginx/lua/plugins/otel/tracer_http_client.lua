local http = require("resty.http")

local _M = {}

local mt = {__index = _M}

------------------------------------------------------------------
-- create a http client used by exporter.
--
-- @address             opentelemetry collector: host:port
-- @timeout             export request timeout second
-- @headers             export request headers
-- @return              http client
------------------------------------------------------------------
function _M.new(address, timeout, headers)
    headers = headers or {}
    headers["Content-Type"] = "application/x-protobuf"

    local uri = address .. "/v1/traces"
    if address:find("http", 1, true) ~= 1 then
        uri = "http://" .. uri
    end

    local self = {uri = uri, timeout = timeout, headers = headers}
    return setmetatable(self, mt)
end

function _M.do_request(self, body)
    local httpc = http.new()
    httpc:set_timeout(self.timeout * 1000)

    local res, err = httpc:request_uri(self.uri, {method = "POST", headers = self.headers, body = body, ssl_verify = false})

    if not res then
        ngx.log(ngx.ERR, "request failed: ", err)
        httpc:close()
        return nil, err
    end

    if res.status ~= 200 then
        ngx.log(ngx.ERR, "request failed: ", res.body)
        httpc:close()
        return nil, "request failed: " .. res.status
    end

    return res, nil
end

return _M
