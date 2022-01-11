local json = require "cjson"
local type = type
local pcall = pcall
local string_sub = string.sub
local getmetatable = getmetatable
local pairs = pairs
local error = error
local ngx = ngx
local sub = ngx.re.sub
local ffi = require("ffi")
local C = ffi.C

ffi.cdef [[
    int memcmp(const void *s1, const void *s2, size_t n);
]]

local _M = {}

_M.null = json.null

function _M.json_encode(data, empty_table_as_object)
    local json_value = nil
    if json.encode_empty_table_as_object then
        json.encode_empty_table_as_object(empty_table_as_object or false) -- 空的table默认为array
    end
    json.encode_sparse_array(true)
    pcall(function(data) -- luacheck: ignore
        json_value = json.encode(data)
    end, data)
    return json_value
end

--- decode json string.
--- if error, return nil
---@param str string json string
---@return table|nil
function _M.json_decode(str)
    local json_value = nil
    pcall(function(str) -- luacheck: ignore
        json_value = json.decode(str)
    end, str)
    return json_value
end

function _M.table_equals(t1, t2, ignore_mt)
    local ty1 = type(t1)
    local ty2 = type(t2)
    if ty1 ~= ty2 then
        return false
    end
    -- non-table types can be directly compared
    if ty1 ~= 'table' and ty2 ~= 'table' then
        return t1 == t2
    end
    -- as well as tables which have the metamethod __eq
    local mt = getmetatable(t1)
    if not ignore_mt and mt and mt.__eq then
        return t1 == t2
    end
    for k1, v1 in pairs(t1) do
        local v2 = t2[k1]
        if v2 == nil or not _M.table_equals(v1, v2) then
            return false
        end
    end
    for k2, v2 in pairs(t2) do
        local v1 = t1[k2]
        if v1 == nil or not _M.table_equals(v1, v2) then
            return false
        end
    end
    return true
end

function _M.tablelength(T)
    local count = 0
    for _ in pairs(T) do
        count = count + 1
    end
    return count
end

_M.DIFF_KIND_CHANGE = "change"
_M.DIFF_KIND_ADD = "add"
_M.DIFF_KIND_REMOVE = "remove"
---
--- find which which key in table are diff,order are ignored.
--- return table describe which key and the reason, such as {"a"="add",b="remove",c="change"}
function _M.get_table_diff_keys(new, old)
    if type(new) ~= "table" or type(old) ~= "table" then
        return {}
    end
    -- hash store key which has diff
    local hash = {}
    -- find new.keys in old
    for k1, v1 in pairs(new) do
        local found = false
        for k2, v2 in pairs(old) do
            if k1 == k2 then
                found = true
                -- ngx.log(ngx.INFO, " find " .. k1 .. "in old")
                if not _M.table_equals(v1, v2) then
                    -- ngx.log(ngx.INFO, k1 .. "in new are not same in old")
                    hash[k1] = _M.DIFF_KIND_CHANGE
                end
                break
            end
        end
        --  key not exist in old,which means been add in new
        if not found then
            hash[k1] = _M.DIFF_KIND_ADD
        end
    end

    -- find what been remove when old->new
    for k1, _ in pairs(old) do
        local found = false
        for k2, _ in pairs(new) do
            if k1 == k2 then
                found = true
                break
            end
        end
        -- key not exist in new,which mean been remove in new
        if not found then
            hash[k1] = _M.DIFF_KIND_REMOVE
        end
    end
    return hash
end

function _M.table_contains(t, v)
    for _, val in ipairs(t) do
        if v == val then
            return true
        end
    end
    return false
end

-- given an Nginx variable i.e $request_uri
-- it returns value of ngx.var[request_uri]
function _M.lua_ngx_var(ngx_var)
    local var_name = string_sub(ngx_var, 2)
    if var_name:match("^%d+$") then
        var_name = tonumber(var_name)
    end

    return ngx.var[var_name]
end

-- {
--    "mode": "http",
--    "session_affinity_attribute": "",
--    "name": "calico-new-yz-alb-09999-eb1a18d0-8d44-4100-bbb8-93db3c02c482",
--    "session_affinity_policy": "",
--    "backends": [
--    {
--        "port": 80,
--        "address": "10.16.12.11",
--        "weight": 100
--    }
--    ]
-- }
function _M.get_nodes(backend)
    local nodes = {}

    for _, endpoint in ipairs(backend.backends) do
        local endpoint_string = endpoint.address .. ":" .. endpoint.port
        nodes[endpoint_string] = endpoint.weight
    end
    return nodes
end

-- http://nginx.org/en/docs/http/ngx_http_upstream_module.html#example
-- CAVEAT: nginx is giving out : instead of , so the docs are wrong
-- 127.0.0.1:26157 : 127.0.0.1:26157 , ngx.var.upstream_addr
-- 200 : 200 , ngx.var.upstream_status
-- 0.00 : 0.00, ngx.var.upstream_response_time
function _M.split_upstream_var(var)
    if not var then
        return nil, nil
    end
    local t = {}
    for v in var:gmatch("[^%s|,]+") do
        if v ~= ":" then
            t[#t + 1] = v
        end
    end
    return t
end

function _M.has_prefix(s, prefix)
    if type(s) ~= "string" or type(prefix) ~= "string" then
        error("unexpected type: s:" .. type(s) .. ", prefix:" .. type(prefix))
    end
    if #s < #prefix then
        return false
    end
    local rc = C.memcmp(s, prefix, #prefix)
    return rc == 0
end

function _M.trim(s, prefix)
    return sub(s, "^" .. prefix, "", "jo")
end

-- @return bool
-- @param table: {} lua table
-- @param path: []string a array of string, use as nested access
-- detect if a nested table has key path,return true if #path==0
-- pure no side effect
function _M.has_key(table, path)
    local cur = table
    for _, p in ipairs(path) do
        if type(cur) == "table" and cur[p] ~= nil then
            cur = cur[p]
        else
            return false
        end
    end
    return true
end

---
--- use path to access s.
--- It return the result of this access and true if have such key,false not have.
---@param s table|nil the object which will be accessed
---@param keys table key list of this table,such as {"a","b","c"}
---@return any,boolean
function _M.access_or(s, keys, default)
    if _M.has_key(s, keys) then
        local v = s
        for _, key in ipairs(keys) do
            v = v[key]
        end
        return v, true
    end
    return default, false
end

function _M.dump(o)
    if type(o) == 'table' then
        local s = '{ '
        for k, v in pairs(o) do
            if type(k) ~= 'number' then
                k = '"' .. k .. '"'
            end
            s = s .. '[' .. k .. '] = ' .. _M.dump(v) .. ','
        end
        return s .. '} '
    else
        return tostring(o)
    end
end

return _M
