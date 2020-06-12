---
--- Created by oilbeater.
--- DateTime: 17/9/15 上午11:59
---
local ipairs = ipairs
local type = type
local tonumber = tonumber
local string_find = string.find
local string_sub = string.sub
local table_remove = table.remove
local table_insert = table.insert
local string_format = string.format
local ngx_re = ngx.re
local ngx_var = ngx.var
local ngx_req = ngx.req
local _M = {}

local bool_op = {
    AND = "AND",
    OR  = "OR",
}

local single_matcher = {
    HOST    = "HOST",
    URL     = "URL",
    SRC_IP  = "SRC_IP",
}

local function parse_single_matcher(matcher)
    if(matcher == "HOST") then
        local host = ngx_var.host
        -- https://www.w3.org/Protocols/rfc2616/rfc2616-sec14.html#sec14.23
        -- client will add port to host if is not a default port
        local s = string_find(host, ":")
        if s ~= nil then
            return string_sub(host, 1, s - 1)
        end
        return host
    elseif(matcher == "URL") then
        return ngx_var.uri
    elseif(matcher == "SRC_IP") then
        local h = ngx_req.get_headers()
        local x_real_ip = h['x-real-ip']
        if x_real_ip then
          return x_real_ip
        end
        local x_forwarded_for = h['x-forwarded-for']
        if x_forwarded_for then
          -- X-Forwarded-For: client, proxy1, proxy2
          local idx = string_find(x_forwarded_for, ",", 1, true)
          if idx then
            return string_sub(x_forwarded_for, 1, idx - 1)
          else
            return x_forwarded_for
          end
        end
        return ngx_var.remote_addr
    else
        return nil
    end
end

local dual_matcher = {
    HEADER  = "HEADER",
    PARAM   = "PARAM",
    COOKIE  = "COOKIE",
}

local function transform_hyphen(key)
    if string_find(key, "-", 1, true) then
        -- string.gsub cannot be JIT, use ngx.re.gsub
        local newstr, _, _ = ngx_re.gsub(key, "-", "_")
        return newstr
    else
        return key
    end
end


local function parse_dual_matcher(matcher, key)
    if(matcher == "HEADER") then
        return ngx_var["http_" .. transform_hyphen(key)]
    elseif(matcher == "PARAM") then
        return ngx_req.get_uri_args()[key]
    elseif(matcher == "COOKIE") then
        local cookie_name = "cookie_" .. key
        return ngx_var[cookie_name]
    else
        return nil
    end
end

-- split args to matcher that to apply operation and args that left
-- return: matcher, args, error
-- example: ["HOST", "www.baidu.com", "baidu.com"] -> req.host, ["www.baidu.com", "baidu.com"], nil
--          ["HEADER", "UID", "1000"]              -> req.header["UID"], ["1000"], nil
local function split_matcher_args(args)
    local matcher = table_remove(args, 1)
    if(single_matcher[matcher]) then
        return parse_single_matcher(matcher), args, nil
    elseif(dual_matcher[matcher]) then
        local key = table_remove(args, 1)
        return parse_dual_matcher(matcher, key), args, nil
    else
        return nil, nil, string_format("unaccepted matcher %s", matcher)
    end
end

local function toboolean(arg)
    if(arg == nil) then
        return false
    end
    if(type(arg) == "boolean") then
        return arg
    end
    return true
end

local function ip_split(str)
    local parts = {}
    local index = string_find(str, "%.", 1)
    while(index ~= nil) do
        local part = string_sub(str, 1, index - 1)
        table_insert(parts, part)
        str = string_sub(str, index + 1)
        index = string_find(str, "%.", 1)
    end
    table_insert(parts, str)
    return parts
end

local function is_str_ip(str)
    local parts = ip_split(str)

    if(#parts ~= 4) then
        return false
    end

    for _, part in ipairs(parts) do
        local num = tonumber(part)
        if(num) then
            if(num < 0 or num > 255) then
                return false
            end
        else
            return false
        end
    end
    return true
end

local function ip_range(matcher, start, finish)
    local m = ip_split(matcher)
    local s = ip_split(start)
    local f = ip_split(finish)
    local function ip_sum(ip)
        return ((tonumber(ip[1]) * 256 + tonumber(ip[2])) * 256 + tonumber(ip[3])) * 256 + tonumber(ip[4])
    end
    return ip_sum(s) <= ip_sum(m) and  ip_sum(m) <= ip_sum(f)
end

local function range(matcher, start, finish)
    if(matcher == nil) then
        return false, nil
    end
    if(tonumber(matcher) and tonumber(start) and tonumber(finish)) then
        return (tonumber(start) <= tonumber(matcher) and tonumber(matcher) <= tonumber(finish)), nil
    elseif(is_str_ip(matcher) and is_str_ip(start) and is_str_ip(finish)) then
        return ip_range(matcher, start, finish)
    else
        return (start <= matcher and matcher <= finish), nil
    end
end

-- all operations return boolean, err
function _M.AND(args)
    for _, v in ipairs(args) do
        if(v == false) then
            return false, nil
        end
    end
    return true, nil
end

function _M.OR(args)
    for _, v in ipairs(args) do
        if(v) then
            return true, nil
        end
    end
    return false, nil
end

function _M.EQ(matcher, args)
    if(#args ~= 1) then
        return false, string_format("EQ except 1 arg, get %d: %s", #args, args)
    end
    return (matcher == args[1]), nil
end

function _M.STARTS_WITH(matcher, args)
    if(matcher == nil) then
        return false, nil
    end
    if(#args ~= 1) then
        return false, string_format("STARTS_WITH except 1 arg, get %d: %s", #args, args)
    end
    if(#matcher < #args[1]) then
        return false, nil
    else
        return string_sub(matcher, 1, #args[1]) == args[1], nil
    end
end

function _M.REGEX(matcher, args)
    if(matcher == nil) then
        return false, nil
    end
    if(#args ~= 1) then
        return false, string_format("REGEX except 1 arg, get %d: %s", #args, args)
    end

    -- enable jit and cache to improve performance https://github.com/openresty/lua-nginx-module#ngxrematch
    local found, _ = ngx_re.match(matcher, args[1], "jo")
    return toboolean(found), nil
end

function _M.EXIST(matcher, args)
    if(matcher == nil) then
        return false, nil
    end
    if(#args ~= 0) then
        return false, string_format("EXIST except 0 arg, get %d: %s", #args, args)
    end
    return toboolean(matcher), nil
end

function _M.IN(matcher, args)
    for _, arg in ipairs(args)
    do
        if(matcher == arg) then
            return true, nil
        end
    end
    return false, nil
end

function _M.RANGE(matcher, args)
    if(matcher == nil) then
        return false, nil
    end
    if(#args ~= 2) then
        return false, string_format("RANGE except 2 arg, get %d: %s", #args, args)
    end

    return range(matcher, args[1], args[2])
end

function _M.eval(op, raw_args)
    if(bool_op[op]) then
        return _M[op](raw_args)
    else
        local matcher, args, err = split_matcher_args(raw_args)
        if(err) then
            return false, err
        end
        return _M[op](matcher, args)
    end
end

return _M
