local base        = require("resty.core.base")
local new_tab     = base.new_tab
local str_sub = string.sub
local str_byte    = string.byte
local insert_tab = table.insert

local ffi         = require "ffi"
local ffi_cdef    = ffi.cdef
local ffi_new     = ffi.new
local C           = ffi.C

ffi_cdef[[
    int inet_pton(int af, const char * restrict src, void * restrict dst);
    uint32_t ntohl(uint32_t netlong);
]]

local AF_INET     = 2
local AF_INET6    = 10
if ffi.os == "OSX" then
    AF_INET6 = 30
end

local _M = {}

local parse_ipv4
do
    local inet = ffi_new("unsigned int [1]")

    function parse_ipv4(ip)
        if not ip then
            return false
        end

        if C.inet_pton(AF_INET, ip, inet) ~= 1 then
            return false
        end

        return C.ntohl(inet[0])
    end
end
_M.parse_ipv4 = parse_ipv4

local parse_ipv6
do
    local inets = ffi_new("unsigned int [4]")

    function parse_ipv6(ip)
        if not ip then
            return false
        end

        if str_byte(ip, 1, 1) == str_byte('[')
                and str_byte(ip, #ip) == str_byte(']') then

            -- strip square brackets around IPv6 literal if present
            ip = str_sub(ip, 2, #ip - 1)
        end

        if C.inet_pton(AF_INET6, ip, inets) ~= 1 then
            return false
        end

        local inets_arr = new_tab(4, 0)
        for i = 0, 3 do
            insert_tab(inets_arr, C.ntohl(inets[i]))
        end
        return inets_arr
    end
end
_M.parse_ipv6 = parse_ipv6


return _M