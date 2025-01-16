-- https://github.com/3scale/lua-resty-url/blob/eebecb494c04681ebfbc85809bd51b223a042e74/src/resty/url.lua
local tostring = tostring
local re_match = ngx.re.match
local concat = table.concat
local tonumber = tonumber
local setmetatable = setmetatable
local re_gsub = ngx.re.gsub
local select = select
local find = string.find
local sub = string.sub
local assert = assert

local _M = {
    _VERSION = '0.3.4',

    ports = {
        https = 443,
        http = 80,
    }
}

function _M.default_port(scheme)
    return _M.ports[scheme]
end

function _M.scheme(url)
    local start = find(url, ':', 1, true)

    if start then
        return sub(url, 1, start - 1), sub(url, start + 1)
    end
end

local core_base = require "resty.core.base"
local core_regex = require "resty.core.regex"
local new_tab = core_base.new_tab
local C = require('ffi').C

local function compile_regex(pattern)
    local compiled, err, flags = core_regex.re_match_compile(pattern, 'joxi')

    assert(compiled, err)

    return compiled, flags
end

local collect_captures = core_regex.collect_captures
local abs_regex, abs_regex_flags = compile_regex([=[
  ^
    (?:(\w+):)? # scheme (1)
    //
    (?:
      ([^:@]+)? # user (2)
      (?:
        :
        ([^@]+)? # password (3)
      )?
    @)?
    ( # host (4)
      [a-z\.\-\d\_]+ # domain
      |
      [\d\.]+ # ipv4
      |
      \[[a-f0-9\:]+\] # ipv6
    )
    (?:
      :(\d+) # port (5)
    )?
    (.*) # path (6)
  $
]=])
local http_regex, http_regex_flags = compile_regex('^https?$')

local function match(str, regex, flags)
    local res = new_tab(regex.ncaptures, regex.name_count)
    if not str then return false, res end

    local rc = C.ngx_http_lua_ffi_exec_regex(regex, flags, str, #str, 0)

    return rc > 0, collect_captures(regex, rc, str, flags, res)
end

local function _match_opaque(scheme, opaque)
    if match(scheme, http_regex, http_regex_flags) then
        return nil, 'invalid endpoint'
    end

    return { scheme, opaque = opaque }
end

local function _transform_match(m)
    m[0] = nil

    if m[6] == '' or m[6] == '/' then m[6] = nil end

    return m
end

function _M.split(url, protocol)
    if not url then
        return nil, 'missing endpoint'
    end

    local scheme, opaque = _M.scheme(url)

    if not scheme then return nil, 'missing scheme' end

    if protocol and not re_match(scheme, protocol, 'oj') then
        return nil, 'invalid protocol'
    end

    local ok, m = match(url, abs_regex, abs_regex_flags)

    if ok then
        return _transform_match(m)
    else
        return _match_opaque(scheme, opaque)
    end
end

function _M.parse(url, protocol)
    local parts, err = _M.split(url, protocol)

    if err then
        return parts, err
    end

    -- https://tools.ietf.org/html/rfc3986#section-3
    return setmetatable({
        scheme = parts[1] or nil,
        user = parts[2] or nil,
        password = parts[3] or nil,
        host = parts[4] or nil,
        port = tonumber(parts[5]),
        path = parts[6] or nil,
        opaque = parts.opaque,
    }, { __tostring = function () return url end })
end

function _M.normalize(uri)
    local regex = [=[
(                     # Capture group

  (?<!/)/             # Look for / that does not follow another /

  # Look for file:///
  (?(?<=\bfile:/)      # if...
    //                    # then look for // right after it
    |                     # else

    # Look for http:// or ftp://, etc.
    (?(?<=:/)            # if [stuff]:/
    /                  # then look for /
    |                   # else

    )
  )
)
/+                   # everything else with / after it
]=]
    return re_gsub(uri, regex, '/', 'jox')
end

function _M.join(...)
    local components = {}

    for i = 1, select('#', ...) do
        components[i] = tostring(select(i, ...))
    end

    return _M.normalize(concat(components, '/'))
end

return _M
