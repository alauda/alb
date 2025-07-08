-- take from ingress-nginx
local string        = string
local string_len    = string.len
local string_format = string.format
local pairs         = pairs
local type          = type
local next          = next
local table         = table

local _M = {}

-- normalize_endpoints takes endpoints as an array of endpoint objects
-- and returns a table where keys are string that's
-- endpoint.address .. ":" .. endpoint.port and values are all true
---@param endpoints Backend[]
local function normalize_endpoints(endpoints)
    local normalized_endpoints = {}

    for _, endpoint in pairs(endpoints) do
        local endpoint_string = string_format("%s:%s", endpoint.address, endpoint.port)
        normalized_endpoints[endpoint_string] = true
    end

    return normalized_endpoints
end

-- diff_endpoints compares old and new
-- and as a first argument returns what endpoints are in new
-- but are not in old, and as a second argument it returns
-- what endpoints are in old but are in new.
-- Both return values are normalized (ip:port).
---@param old Backend[]
---@param new Backend[]
function _M.diff_endpoints(old, new)
    local endpoints_added, endpoints_removed = {}, {}
    local normalized_old = normalize_endpoints(old)
    local normalized_new = normalize_endpoints(new)

    for endpoint_string, _ in pairs(normalized_old) do
        if not normalized_new[endpoint_string] then
            table.insert(endpoints_removed, endpoint_string)
        end
    end

    for endpoint_string, _ in pairs(normalized_new) do
        if not normalized_old[endpoint_string] then
            table.insert(endpoints_added, endpoint_string)
        end
    end

    return endpoints_added, endpoints_removed
end

function _M.is_blank(str)
    return str == nil or string_len(str) == 0
end

-- this implementation is taken from:
-- https://github.com/luafun/luafun/blob/master/fun.lua#L33
-- SHA: 04c99f9c393e54a604adde4b25b794f48104e0d0
local function deepcopy(orig)
    local orig_type = type(orig)
    local copy
    if orig_type == 'table' then
        copy = {}
        for orig_key, orig_value in next, orig, nil do
            copy[deepcopy(orig_key)] = deepcopy(orig_value)
        end
    else
        copy = orig
    end
    return copy
end
_M.deepcopy = deepcopy

return _M
