local _M = {}

function _M.test()
    ngx.log(ngx.INFO, "life: test https upgrade")
	local F = require("F");local u = require("util");local h = require("test-helper");local httpc = require("resty.http").new();
    local res, err = httpc:request_uri("https://127.0.0.1/lua",{
        headers = {
            ["Upgrade"] = "upgrade",
            ["xxx"] = "lua",
        },
        ssl_verify = false
    })
    local lua_contains = string.find(res.body, "upgrade") ~= nil
	u.log(F"\nvia httpc\n  {res.body} \n has-header {lua_contains}\n")

    local out1 = u.shell_curl([[  curl --http1.1  -k -H "xxx: curl"  "Upgrade: websocket"  "https://127.0.0.1/curl" ]])
    local curl_contains = string.find(out1, "upgrade") ~= nil
	u.log(F"\nvia cur-1.1\n {out}\n has-header {curl_contains}\n ")

    do 
        local out = u.shell_curl([[  curl  -k -H "xxx: curl"    -H "Upgrade: websocket"  "https://127.0.0.1/curl" ]])
        local curl_11_https_with_http2 = string.find(out, "upgrade") ~= nil
        u.log("\ncurl 1.1 to https with http2 "..tostring(curl_11_https_with_http2).."\n")
        -- https://trac.nginx.org/nginx/ticket/1992
        h.assert_not_contains(out,"upgrade",F"curl 1.1 to https with http2")
    end
    do
        local out,err = u.shell_curl([[  curl -v -k -H "xxx: curl" -H "Upgrade: websocket"  "https://127.0.0.1:3443/curl" ]])
        local curl_https_without_http2 = string.find(out, "upgrade") ~= nil
        u.log("\nout  "..tostring(out).."\nerr\n"..tostring(err).."\n")
        u.log("\ncurl 1.1 to https without http2 "..tostring(curl_https_without_http2).."\n")
        h.assert_contains(out,"upgrade",F"curl 1.1 to https without http2")
    end

    do
        local out = u.shell_curl([[  curl -k -H "xxx: curl" https://127.0.0.1:3443/curl" ]])
        local curl_https_without_http2_without_header = string.find(out, "upgrade") ~= nil
        u.log("\n curl_https_without_http2_without_header "..tostring(curl_https_without_http2_without_header).."\n")
        h.assert_not_contains(out,"upgrade")
    end
end
return _M