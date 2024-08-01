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
local string_format = string.format
local ip_util = require "utils.ip"
local ngx = ngx
local ngx_re = ngx.re
local _M = {}

local bool_op = {
    AND = "AND",
    OR  = "OR",
}

local single_matcher = {
    HOST    = "HOST",
    URL     = "URL",
    SRC_IP  = "SRC_IP",
    METHOD  = "METHOD",
}

local function parse_single_matcher(matcher)

    if(matcher == "METHOD") then
		return ngx.ctx.alb_ctx.var["method"]
	end
    if(matcher == "HOST") then
        local host = ngx.ctx.alb_ctx.var.host
        -- https://www.w3.org/Protocols/rfc2616/rfc2616-sec14.html#sec14.23
        -- client will add port to host if is not a default port
        local s = string_find(host, ":")
        if s ~= nil then
            return string_sub(host, 1, s - 1)
        end
        return host
    elseif(matcher == "URL") then
        return ngx.ctx.alb_ctx.var.uri
    elseif(matcher == "SRC_IP") then
        local x_real_ip = ngx.ctx.alb_ctx.var["http_" .. "x_real_ip"]
        if x_real_ip then
          return x_real_ip
        end
        local x_forwarded_for = ngx.ctx.alb_ctx.var["http_" .. "x_forwarded_for"]
        if x_forwarded_for then
          -- X-Forwarded-For: client, proxy1, proxy2
          local idx = string_find(x_forwarded_for, ",", 1, true)
          if idx then
            return string_sub(x_forwarded_for, 1, idx - 1)
          else
            return x_forwarded_for
          end
        end
        return ngx.ctx.alb_ctx.var.remote_addr
    else
        return nil
    end
end

local dual_matcher = {
    HEADER  = "HEADER",
    PARAM   = "PARAM",
    COOKIE  = "COOKIE",
}

local function parse_dual_matcher(matcher, key)
    if(matcher == "HEADER") then
        return ngx.ctx.alb_ctx.var["http_" .. key]
    elseif(matcher == "PARAM") then
        return ngx.ctx.alb_ctx.var["param_" .. key]
    elseif(matcher == "COOKIE") then
        return ngx.ctx.alb_ctx.var["cookie_" .. key]
    end
    return nil
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

local function to_boolean(arg)
    if(arg == nil) then
        return false
    end
    if(type(arg) == "boolean") then
        return arg
    end
    return true
end

local function range(matcher, start, finish)
    if(matcher == nil) then
        return false, nil
    end
    local tm = type(matcher)
    local ts = type(start)
    local tf = type(finish)
    if not(tm == ts and tm == tf) then
        return false, "not same type"
    end

    if not(tm == "number" or tm == "string") then
        return false, "only number type and string type can compare"
    end
    if tm == "number" then
        return (tonumber(start) <= tonumber(matcher) and tonumber(matcher) <= tonumber(finish)), nil
    end
    -- ip compare
    if ip_util.parse_ipv4(matcher) then
        local ip4_m = ip_util.parse_ipv4(matcher)
        local ip4_s = ip_util.parse_ipv4(start)
        local ip4_e = ip_util.parse_ipv4(finish)
        if not(ip4_s and ip4_e) then
            return false, "invalid ip address"
        end
        return ip4_s <= ip4_m and ip4_m <= ip4_e, nil
    elseif ip_util.parse_ipv6(matcher) then
        local ip6_m = ip_util.parse_ipv6(matcher)
        local ip6_s = ip_util.parse_ipv6(start)
        local ip6_e = ip_util.parse_ipv6(finish)
        if not(ip6_s and ip6_e) then
            return false, "invalid ip address"
        end

        for i = 1, 4 do
            if ip6_s[i] < ip6_m[i] and ip6_m[i] < ip6_e[i] then
                return true
            elseif ip6_s[i] > ip6_m[i] or ip6_m[i] > ip6_e[i] then
                return false
            end
        end
        return true, nil
    end
    -- string compare
    return (start <= matcher and matcher <= finish), nil
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
        -- string.sub(s,1,j) returns a prefix of s with length j
        return string_sub(matcher, 1, #args[1]) == args[1], nil
    end
end

function _M.ENDS_WITH(matcher, args)
    if(matcher == nil) then
        return false, nil
    end
    if(#args ~= 1) then
        return false, string_format("ENDS_WITH except 1 arg, get %d: %s", #args, args)
    end
    if(#matcher < #args[1]) then
        return false, nil
    else
        -- generic-host: args[1] is in format as *.foo.bar
        -- string.sub(s, -i) returns a suffix of s with length i
        return (string_sub(matcher, -(#args[1] - 1)) == string_sub(args[1], 2)) and (string_find(string_sub(matcher,1,#matcher-#args[1]),'%.') == nil), nil
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
    return to_boolean(found), nil
end

function _M.EXIST(matcher, args)
    if(matcher == nil) then
        return false, nil
    end
    if(#args ~= 0) then
        return false, string_format("EXIST except 0 arg, get %d: %s", #args, args)
    end
    return to_boolean(matcher), nil
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
        local ret = _M[op](matcher, args)
        -- ngx.log(ngx.ERR, "eval: ", op, " ", matcher," ret ", ret)
        return ret
    end
end

return _M
