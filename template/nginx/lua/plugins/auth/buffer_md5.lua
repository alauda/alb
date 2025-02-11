local ffi = require "ffi"

local C = ffi.C
local ffi_new = ffi.new
local ffi_string = ffi.string
local ngx = ngx
local subsystem = ngx.config.subsystem


local ngx_lua_ffi_md5_bin
if subsystem == "http" then
    ffi.cdef [[
    void ngx_http_lua_ffi_md5_bin(const unsigned char *src, size_t len,
                                  unsigned char *dst);
    ]]
    ngx_lua_ffi_md5_bin = C.ngx_http_lua_ffi_md5_bin
end

local MD5_DIGEST_LEN = 16
local md5_buf = ffi_new("unsigned char[?]", MD5_DIGEST_LEN)

local _m = {}

-- 17ms -> 14ms
_m.md5_bin = function (sb)
    local ptr, len = sb:ref()
    ngx_lua_ffi_md5_bin(ptr, len, md5_buf)
    return ffi_string(md5_buf, MD5_DIGEST_LEN)
end

return _m
