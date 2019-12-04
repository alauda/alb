local json = require "cjson"
local type = type
local pcall = pcall
local string_sub = string.sub
local getmetatable = getmetatable
local pairs = pairs

local _M = {}

function _M.json_encode(data, empty_table_as_object )
  local json_value = nil
  if json.encode_empty_table_as_object then
    json.encode_empty_table_as_object(empty_table_as_object or false) -- 空的table默认为array
  end
  json.encode_sparse_array(true)
  pcall(function (data) json_value = json.encode(data) end, data)
  return json_value
end

function _M.json_decode(str)
	local json_value = nil
	pcall(function (str) json_value = json.decode(str) end,str)
	return json_value
end

function _M.table_equals(t1, t2, ignore_mt)
    local ty1 = type(t1)
    local ty2 = type(t2)
    if ty1 ~= ty2 then return false end
    -- non-table types can be directly compared
    if ty1 ~= 'table' and ty2 ~= 'table' then return t1 == t2 end
    -- as well as tables which have the metamethod __eq
    local mt = getmetatable(t1)
    if not ignore_mt and mt and mt.__eq then return t1 == t2 end
    for k1,v1 in pairs(t1) do
        local v2 = t2[k1]
        if v2 == nil or not _M.table_equals(v1,v2) then return false end
    end
    for k2,v2 in pairs(t2) do
        local v1 = t1[k2]
        if v1 == nil or not _M.table_equals(v1,v2) then return false end
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

-- given an Nginx variable i.e $request_uri
-- it returns value of ngx.var[request_uri]
function _M.lua_ngx_var(ngx_var)
    local var_name = string_sub(ngx_var, 2)
    if var_name:match("^%d+$") then
        var_name = tonumber(var_name)
    end

    return ngx.var[var_name]
end


--{
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
--}
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
      t[#t+1] = v
    end
  end
  return t
end

return _M
