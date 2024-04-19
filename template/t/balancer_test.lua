---@diagnostic disable: need-check-nil, unreachable-code, undefined-global
local httpc = require("resty.http").new()
local function curl(url, cfg)
    if cfg == nil then
        cfg = {}
        cfg["headers"] = {}
    end
    local res, err = httpc:request_uri(url, {method = "GET", headers = cfg and cfg.headers})
    if not res then
        ngx.log(ngx.ERR, "request failed: ", err)
        return
    end
    local status = res.status
    local body = string.gsub(res.body, "%s+", "")
    return body
end

local function is_average(table, stand, offset)
    for backend, count in pairs(table) do
        if count > (stand + offset) or count < (stand - offset) then
            ngx.log(ngx.WARN, "not average " .. backend .. " count " .. count)
            return false
        end
    end
    return true
end

local _M = {}
function _M.roundrobin()
    local bucket = {}
    local count = 1000
    local backend_size = 5
    for i = 1, count do
        local backend = curl("http://127.0.0.1")
        if bucket[backend] then
            bucket[backend] = bucket[backend] + 1
        else
            bucket[backend] = 1
        end
    end
    for backend, count in pairs(bucket) do
        ngx.log(ngx.WARN, "backend " .. backend .. " count " .. count)
    end
    local ret = is_average(bucket, count / backend_size, 10)
    return ret
end

function _M.sticky_header(header_name)
    -- same header should get same backend
    local headers = {[header_name] = "A"}
    local backendForA = curl("http://127.0.0.1", {headers = headers})
    for i = 1, 10 do
        local headers = {[header_name] = "A"}
        local headers = {[header_name] = "A"}
        local newBackend = curl("http://127.0.0.1", {headers = headers})
        if backendForA ~= newBackend then return false end
    end
    -- different headers should get different backend
    local headers = {[header_name] = "B"}
    local backendForB = curl("http://127.0.0.1", {headers = headers})
    for i = 1, 10 do
        local headers = {[header_name] = "B"}
        local newBackend = curl("http://127.0.0.1", {headers = headers})
        if backendForB ~= newBackend then return false end
    end
    if backendForB ~= backendForA then return true end

    -- no header, use a random backend
    local bucket = {}
    local count = 100
    local backend_size = 5
    for i = 1, count do
        local backend = curl("http://127.0.0.1")
        if bucket[backend] then
            bucket[backend] = bucket[backend] + 1
        else
            bucket[backend] = 1
        end
    end
    for backend, count in pairs(bucket) do
        ngx.log(ngx.WARN, "backend " .. backend .. " count " .. count)
    end
    local ret = is_average(bucket, count / backend_size, 10)
    return ret
end

function _M.sticky_cookie(cookie_name)
    if cookie_name == "" then
        cookie_name = "JSESSIONID"
    end

    -- same cookie should get same backend
    local headers = {["Cookie"] = string.format("%s=A", cookie_name)}
    local backendForA = curl("http://127.0.0.1", {headers = headers})
    for i = 1, 10 do
        local headers = {["Cookie"] = string.format("%s=A", cookie_name)}
        local newBackend = curl("http://127.0.0.1", {headers = headers})
        if backendForA ~= newBackend then return false end
    end
    -- different cookie should get different backend
    local headers = {["Cookie"] = string.format("%s=B", cookie_name)}
    local backendForB = curl("http://127.0.0.1", {headers = headers})
    for i = 1, 10 do
        local headers = {["Cookie"] = string.format("%s=B", cookie_name)}
        local newBackend = curl("http://127.0.0.1", {headers = headers})
        if backendForB ~= newBackend then return false end
    end
    if backendForB ~= backendForA then return true end

    -- no cookie, use a random backend
    local bucket = {}
    local count = 100
    local backend_size = 5
    for i = 1, count do
        local backend = curl("http://127.0.0.1")
        if bucket[backend] then
            bucket[backend] = bucket[backend] + 1
        else
            bucket[backend] = 1
        end
    end
    for backend, count in pairs(bucket) do
        ngx.log(ngx.WARN, "backend " .. backend .. " count " .. count)
    end
    if not is_average(bucket, count / backend_size, 10) then return false end

    -- could not find cookie key from client, set it to null
    local res, err = httpc:request_uri("http://127.0.0.1", {method = "GET"})
    for k, v in pairs(res.headers) do
        ngx.log(ngx.WARN, "k " .. k .. " v " .. tostring(v))
    end
    local cookie = res.headers["Set-Cookie"]
    if cookie_name == "" then
        return string.find(cookie, "JSESSIONID")
    else
        return string.find(cookie, cookie_name)
    end

    return ret

end

function _M.test_balancer(policy, attribute)
    local ret = _M[policy](attribute)
    ngx.log(ngx.WARN, "test " .. policy .. " is " .. tostring(ret))
    return ret
end
return _M
