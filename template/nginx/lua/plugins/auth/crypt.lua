local bit = require("bit")
local buffer = require("string.buffer")
local buffer_md5 = require("plugins.auth.buffer_md5")
local _m = {}

-- TODO use ffi?
-- thanks for https://github.com/Tblue/pyapr1/blob/master/apr1.py
---comment
---@param pass string
---@param salt string
---@return string hash
_m.apr1 = function (pass, salt)
    local make_int = _m.make_int
    local to64 = _m.to64

    local hash1_data = pass .. "$apr1$" .. salt
    local hash2_data = hash1_data
    -- local hash1 = ngx.md5_bin(hash1_data)
    -- ngx.log(ngx.INFO, "hash 1 " .. str.to_hex(hash1))
    local sandwich = ngx.md5_bin(pass .. salt .. pass)
    -- ngx.log(ngx.INFO, "sandwich " .. str.to_hex(sandwich))
    local MD5_DIGEST_SIZE = 16
    local n_dig, n_rem = _m.divmod(#pass, MD5_DIGEST_SIZE)
    for _ = 1, n_dig, 1 do
        hash2_data = hash2_data .. sandwich
    end
    hash2_data = hash2_data .. sandwich:sub(1, n_rem)

    -- 嵌入长度二进制编码
    local i = #pass
    while i > 0 do
        if bit.band(i, 1) == 1 then
            hash2_data = hash2_data .. "\0"
        else
            hash2_data = hash2_data .. pass:sub(1, 1)
        end
        i = bit.rshift(i, 1) -- 右移一位
    end
    local hash2 = ngx.md5_bin(hash2_data)
    -- ngx.log(ngx.INFO, "hash2 " .. str.to_hex(hash2))
    local final = hash2
    -- lua loop are [] not [)
    -- use restry_md5 and you will find it get slower..
    local max_len = (#pass + #salt + #final) * 2
    local step = buffer.new(max_len)
    for i = 0, 999, 1 do
        step:reset()
        -- use .. to combind byte and use ngx.md5_bin is faster than table concat + resty_md5..
        -- use string buffer but still not obvious perf improvement..
        -- use buffer aware md5. qps 17ms -> 14ms
        if bit.band(i, 1) == 1 then
            step:put(pass)
        else
            step:put(final)
        end
        if i % 3 ~= 0 then
            step:put(salt)
        end
        if i % 7 ~= 0 then
            step:put(pass)
        end
        if bit.band(i, 1) == 1 then
            step:put(final)
        else
            step:put(pass)
        end
        final = buffer_md5.md5_bin(step)
    end
    -- base64
    local f1 = make_int(final, 0, 6, 12)
    local f2 = make_int(final, 1, 7, 13)
    local f3 = make_int(final, 2, 8, 14)
    local f4 = make_int(final, 3, 9, 15)
    local f5 = make_int(final, 4, 10, 5)
    local f6 = make_int(final, 11)
    -- ngx.log(ngx.INFO, "f1 " .. f1 .. " " .. to64(f1, 4))
    -- ngx.log(ngx.INFO, "f2 " .. f2 .. " " .. to64(f2, 4))
    -- ngx.log(ngx.INFO, "f3 " .. f3 .. " " .. to64(f3, 4))
    -- ngx.log(ngx.INFO, "f4 " .. f4 .. " " .. to64(f4, 4))
    -- ngx.log(ngx.INFO, "f5 " .. f5 .. " " .. to64(f5, 4))
    -- ngx.log(ngx.INFO, "f6 " .. f6 .. " " .. to64(f6, 2))

    return to64(f1, 4) .. to64(f2, 4) .. to64(f3, 4) .. to64(f4, 4) .. to64(f5, 4) .. to64(f6, 2)
end

_m.divmod = function (dividend, divisor)
    local quotient = math.floor(dividend / divisor) -- 商
    local remainder = dividend % divisor            -- 余数
    return quotient, remainder
end

_m.make_int = function (data, ...)
    local indexes = { ... }
    local r = 0
    for i, idx in ipairs(indexes) do
        -- 提取字节并左移，然后按位或操作
        r = bit.bor(r, bit.lshift(string.byte(data, idx + 1), 8 * (#indexes - i)))
    end
    return r
end
-- apr1的base64..并不是通常的base64..
_m.to64 = function (data, n_out)
    local chars = "./0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
    local out = ""

    for _ = 1, n_out do
        -- 提取低 6 位并映射到字符
        local index = bit.band(data, 0x3F) + 1 -- Lua 索引从 1 开始
        out = out .. chars:sub(index, index)
        -- 右移 6 位
        data = bit.rshift(data, 6)
    end
    return out
end

return _m
