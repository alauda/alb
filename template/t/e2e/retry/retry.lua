local _M = {}
local F = require("F");
local u = require("util");
local h = require("test-helper");
local httpc = require("resty.http").new();

local function get_string_count(msg, module)
    local map = {
        ["http"] = "./template/servroot/logs/error.http.log",
        ["stream"] = "./template/servroot/logs/error.stream.log",
    }
    local cmd = string.format([[bash -c "cat %s |grep '%s' | wc -l"]], map[module], msg)
    local final, err = u.shell(cmd)
    if err ~= nil and err ~= "" then
        h.fail("get string count fail")
    end
    return tonumber(final)
end

function _M.test()
    local msg = "connect() failed (111: Connection refused)"
    do
        local orgin = get_string_count(msg, "http")
        local res, err = httpc:request_uri("http://127.0.0.1/ping")
        h.assert_eq(res.status, 502)
        local final = get_string_count(msg, "http")
        h.assert_eq(final - orgin, 5, F("http {orgin} {final}"))
    end
    do
        local orgin = get_string_count(msg, "stream")
        local res, err = httpc:request_uri("http://127.0.0.1:81/ping")
        h.assert_eq(err, "connection reset by peer")
        local final = get_string_count(msg, "stream")
        h.assert_eq(final - orgin, 5, F "stream {orgin} {final}")
    end
end

return _M
