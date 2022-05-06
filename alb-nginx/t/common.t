use strict;
use warnings;
use t::Alauda;
use Test::Nginx::Socket 'no_plan';

no_shuffle();
no_root_location();
run_tests();

# this test file used to test function inside common.lua

__DATA__

=== TEST 1: common stuff should ok
--- log_level: info
--- http_config
server {
    listen 9999;
    location /t {
        content_by_lua_block {
            package.path = '/t/?.lua;'.."/t/lib/?.lua;" .. package.path;
            local common = require("utils/common")
            local h = require("test-helper")

            -- json decode should ok
            local v = common.json_decode("")
            h.assert_eq(v,nil)

            local v = common.json_decode(nil)
            h.assert_eq(v,nil)

            local v = common.json_decode("{}")
            h.assert_eq(v,{})

            -- json encode should ok
            local json_str = common.json_encode(nil)
            h.assert_eq(json_str,"null")

            local json_str = common.json_encode({},false)
            h.assert_eq(json_str,"[]")

            local json_str = common.json_encode({},true)
            h.assert_eq(json_str,"{}")


            -- json encode and decode loop
            local v = {a={b={c="c"}}}
            local json_str = common.json_encode(v,true)
            h.assert_eq(json_str,'{"a":{"b":{"c":"c"}}}')

            local ret_v =  common.json_decode(json_str)
            h.assert_eq(common.table_equals(v,ret_v),true)


            -- table_equals should ok
            h.assert_eq(common.table_equals({},{}),true)
            h.assert_eq(common.table_equals({a=1},{}),false)

            -- table has key should ok
            h.assert_eq(common.has_key({a={b={c=2}}},{"a","b","c"}),true)
            h.assert_eq(common.has_key({a={b={d=2}}},{"a","b","c"}),false)

            -- table access_or should ok
            local v,find = common.access_or({a={b={d=2}}},{"a","b","c"},1)
            h.assert_eq(v,1)
            h.assert_eq(find,false)

            local v,find = common.access_or({a={b={c=2}}},{"a","b","c"},1)
            h.assert_eq(v,2)
            h.assert_eq(find,true)

            local v,find = common.access_or({a={b={c={d={e="e"}}}}},{"a","b","c"},1)
            h.assert_eq(v,{d={e="e"}})
            h.assert_eq(find,true)

            ngx.say("success");
        }
    }
}
--- server_port: 9999
--- request
    GET /t
--- response_body
success
--- no_error_log
[error]

=== TEST 2: get_table_diff_keys should ok
--- log_level: info
--- http_config
server {
    listen 9999;
    location /t {
        content_by_lua_block {
            package.path = '/t/?.lua;'.."/t/lib/?.lua;" .. package.path;
            local common = require("utils/common");
            local h = require("test-helper");

            local old = {a=1,b=1,c=1}
            local new = {a=1,b=2,c=1}
            local diff = common.get_table_diff_keys(new,old)
            h.assert_eq(diff,{b="change"})

            local old = {a={["a1"]="a"},b={["b1"]="b"},c= {["c1"]="c"} }
            local new = {               b={["b1"]="b"},c= {["c1"]="c1"},d= {["d1"]="d"} }
            local diff = common.get_table_diff_keys(new,old)
            h.assert_eq(diff,{a="remove",c="change",d="add"})

            ngx.say("success");
        }
    }
}
--- server_port: 9999
--- request
    GET /t
--- response_body
success
--- no_error_log
[error]

=== TEST 3: regex should ok 
--- log_level: info
--- lua_test
			local F = require("F");local u = require("util");local h = require("test-helper");
    		local found, _ = ngx.re.match("/test", "/test", "jo")
			ngx.log(ngx.NOTICE,"1 found "..u.inspect(found))
    		local found, _ = ngx.re.match("/test", "/(?!(login)|(ping))test", "jo")
			ngx.log(ngx.NOTICE,"2 found "..u.inspect(found))
    		local found, _ = ngx.re.match("/ping", "/(?!(login)|(ping))test", "jo")
			ngx.log(ngx.NOTICE,"3 found "..u.inspect(found))
--- response_body: ok
