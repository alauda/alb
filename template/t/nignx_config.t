use strict;
use warnings;
use t::Alauda;
use Test::Nginx::Socket 'no_plan';

no_shuffle();
no_root_location();
run_tests();

__DATA__

=== TEST 1: valid nginx config ok
--- log_level: info
--- alb_stream_server_config
    server {
        listen 9002 so_keepalive=on;
        content_by_lua_block {
          ngx.print("ok");
        }
    }
    server {
        listen 9001 so_keepalive=30m::10;
        content_by_lua_block {
          ngx.print("ok");
        }
    }
    server {
        listen 9003 so_keepalive=30m:1s:10;
        content_by_lua_block {
          ngx.print("ok");
        }
    }
--- lua_test
    ngx.log(ngx.INFO, "ok")
