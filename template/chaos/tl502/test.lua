local _M = {}
local u = require "util"

function _M.as_backend()
    ngx.say "ok"

end
function _M.as_backend_1_0()
    ngx.say "ok"
end
function _M.as_backend_1_1()
    ngx.say "ok"
end

function _M.test()
    -- do
    --     local httpc = require"resty.http".new()
    --     local res, err = httpc:request("http://127.0.0.1:80/1_0/1")
    --     u.logs(res, err)
    --     local res, err = httpc:request("http://127.0.0.1:80/1_0/2")
    --     u.logs(res, err)
    -- end
    -- do
    --     local httpc = require"resty.http".new()
    --     local res, err = httpc:request("http://127.0.0.1:80/1_1/1")
    --     u.logs(res, err)
    --     local res, err = httpc:request_uri("http://127.0.0.1:80/1_1/2")
    --     u.logs(res, err)
    -- end
    do
        local httpc = require"resty.http".new()
        local res, err = httpc:request_uri("http://127.0.0.1:80/default/1", {method = "POST", body = "xx"})
        u.logs(res, err)
        local res, err = httpc:request_uri("http://127.0.0.1:80/default/2", {method = "POST", body = "xx"})
        u.logs(res, err)

        ngx.sleep(3000)
    end
end
return _M
