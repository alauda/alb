use strict;
use warnings;
use t::Alauda;
use Test::Nginx::Socket 'no_plan';

no_shuffle();
no_root_location();
run_tests();

__DATA__

=== TEST 1: udp ping/pong should ok
--- log_level: info
--- alb_stream_server_config
    server {
        listen 9001 udp;
        content_by_lua_block {
            ngx.log(ngx.INFO,"udp socket connect")
            local sock,err = ngx.req.socket()
            local data, err = sock:receive()
            if err ~= nil then
                sock:send("err "..tostring(err))
            end
            sock:send(data)
        }
    }
--- http_config
server {
    listen 9000;
    location /t {
        content_by_lua_block {
            ngx.log(ngx.INFO,"on test")
            local socket = ngx.socket
            local udp = socket.udp()
            ngx.log(ngx.INFO,"init udp client")
            local ok, err = udp:setpeername("127.0.0.1", 9001)
            if not ok then
                ngx.log(ngx.ERR,"failed to connect: ", err)
                return
            end
            ngx.log(ngx.INFO,"connect to udp success")
            udp:send("wake up") -- if we do not send anything it will not trigger content_by_lua_block
            local data, err = udp:receive()
            if not data then
                ngx.log(ngx.ERR,"failed to receive data: ", err)
                return
            end
            ngx.log(ngx.INFO,"received ", #data, " bytes: ", data)
            udp:close()
            if data == "wake up" then
                ngx.print("success")
            end
        }
    }
}
--- server_port: 9000
--- request: GET /t
--- response_body: success
--- no_error_log
[error]

=== TEST 2: udp ping/pong via alb should ok
--- log_level: info
--- alb_stream_server_config
    server {
        listen 9001 udp;
        content_by_lua_block {
            ngx.log(ngx.INFO,"udp socket connect")
            local sock,err = ngx.req.socket()
            local data, err = sock:receive()
            if err ~= nil then
                sock:send("err "..tostring(err))
            end
            sock:send(data)
        }
    }
--- policy
{
   "stream":{"udp":{"82":[{"upstream":"test-udp-upstream-1"}]}},
   "backend_group":[
      {
         "name":"test-udp-upstream-1",
         "mode":"udp",
         "backends":[
            {
               "address":"127.0.0.1",
               "port":9001,
               "weight":100
            }
         ]
      }
   ]
}
--- http_config
server {
    listen 9000;
    location /t {
        content_by_lua_block {
            ngx.log(ngx.INFO,"on test")
            local socket = ngx.socket
            local udp = socket.udp()
            local ok, err = udp:setpeername("127.0.0.1",82)
            if not ok then
                ngx.log(ngx.ERR,"failed to connect: ", err)
                return
            end
            ngx.log(ngx.INFO,"connect to udp success")
            local msg = "wake up"
            udp:send(msg)
            local data, err = udp:receive()
            if not data then
                ngx.log(ngx.ERR,"failed to receive data: ", err)
                return
            end
            ngx.log(ngx.INFO,"received ", #data, " bytes: ", data)
            udp:close()
            if data == msg then
                ngx.print("success")
            end
        }
    }
}
--- server_port: 9000
--- request: GET /t
--- response_body: success
--- no_error_log
[error]
