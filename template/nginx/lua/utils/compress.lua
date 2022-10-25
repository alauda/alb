local _M = {}
local zlib = require('lua-ffi-zlib.lib.ffi-zlib')
-- decompress zlib format data from file
-- arg fpath: string, path of the file
-- ret out: string|nil,err: string| nil
local _decompress_from_file = function(fpath)
    local f = io.open(fpath, "rb")
    if f == nil then
        return nil, "decompress file is nill"
    end
    local input = function(bufsize)
        local d = f:read(bufsize)
        if d == nil then
            return nil
        end
        return d
    end
    local out = {}
    local output = function(data)
        table.insert(out, data)
    end
    local ok, err = zlib.inflateGzip(input, output)
    if not ok then
        return nil,err
    end
    return table.concat(out),nil
end

-- decompress zlib format data from file
-- arg fpath: string, path of the file
-- ret out: string|nil,err: string| nil
function _M.decompress_from_file(fpath)
    local ok,out,err = pcall(_decompress_from_file,fpath)
    if not ok then
        return nil,"decompress fail "..out
    end
    if err ~= nil then
        return nil,"decompress fail "..err
    end
    return out,nil
end

return _M