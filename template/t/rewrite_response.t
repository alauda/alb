use strict;
use warnings;
use t::Alauda;
use Test::Nginx::Socket 'no_plan';
use Test::Nginx::Socket;

our $policy = <<'_EOC_';
{
  "certificate_map": {},
  "http": {"tcp": {
      "80": [
        {
          "rule": "",
          "internal_dsl": [
            [
              "STARTS_WITH",
              "URL",
              "/set-header"
            ]
          ],
          "upstream": "test-upstream-1",
          "config": {
              "rewrite_response": {
                  "headers": {
                      "a": "b"
                  }
              }
          }
        },
        {
          "rule": "",
          "internal_dsl": [
            [
              "STARTS_WITH",
              "URL",
              "/remove-header"
            ]
          ],
          "upstream": "test-upstream-1",
          "config": {
              "rewrite_response": {
                  "headers_remove": [
                      "test","test1"
                  ]
              }
          }
        },
        {
          "rule": "",
          "internal_dsl": [
            [
              "STARTS_WITH",
              "URL",
              "/add-header"
            ]
          ],
          "upstream": "test-upstream-1",
          "config": {
              "rewrite_response": {
                  "headers_add": {
                      "test": ["2","3"] ,
                      "a1": ["b1"] ,
                      "a2": ["b1","b2"] 
                  }
              }
          }
        }
        ]}},
  "backend_group": [
    {
      "name": "test-upstream-1",
      "mode": "http",
      "backends": [
        {
          "address": "127.0.0.1",
          "port": 9999,
          "weight": 100
        }
      ]
    }]
}
_EOC_
our $http_config = <<'_EOC_';
server {
    listen 9999;
    location / {
       content_by_lua_block {
           for k,v in pairs(ngx.req.get_headers()) do
               local find,last = string.find(k, "upstream-",1,true) 
               if find ~=nil then
                       local newk = string.sub(k,last+1,#k) 
                    ngx.log(ngx.ERR,"nk "..newk)
                    ngx.header[newk] = v
               end
           end
           ngx.print("ok")
      }
    }
}
_EOC_


log_level("info");
no_shuffle();
no_root_location();
run_tests();


__DATA__

=== TEST 1: test rewrite_response headers echo-server
--- policy eval: $::policy
--- http_config eval: $::http_config
--- lua_test
    local F = require("F");local u = require("util");local h = require("test-helper");
    do
        local httpc = require("resty.http").new()
        local res, err = httpc:request_uri("http://127.0.0.1:9999/t",{
            headers = {
                ["upstream-test"] = "sss"
            }
        })
        h.assert_eq(res.headers.test,"sss")
        h.assert_eq(res.body,"ok")
    end

=== TEST 2: test rewrite_response headers set 
--- policy eval: $::policy
--- http_config eval: $::http_config
--- lua_test
    local F = require("F");local u = require("util");local h = require("test-helper");
    local httpc = require("resty.http").new()

    local res, err = httpc:request_uri("http://127.0.0.1:80/set-header",{
        headers = {
            ["test"] = "sss"
        }
    })
    h.assert_eq(res.headers.a,"b")
    h.assert_eq(res.headers.test,nil)


=== TEST 2: test rewrite_response headers remove
--- policy eval: $::policy
--- http_config eval: $::http_config
--- lua_test
    local F = require("F");local u = require("util");local h = require("test-helper");
    local httpc = require("resty.http").new()

    -- remove header ok
    local res, err = httpc:request_uri("http://127.0.0.1:80/remove-header",{
        headers = {
            ["upstream-test"] = "sss",
			["upstream-test1"] = "sss1",
            ["upstream-test2"] = "sss2"
        }
    })
    h.assert_eq(res.headers.test,nil)
    h.assert_eq(res.headers.test1,nil)
    h.assert_eq(res.headers.test2,"sss2")

=== TEST 3: test rewrite_response headers add
--- policy eval: $::policy
--- http_config eval: $::http_config
--- lua_test
    local F = require("F");local u = require("util");local h = require("test-helper");
    -- add header ok
    local out, verbose = u.shell_curl([[ curl -v -s http://127.0.0.1:80/add-header -H "upstream-test: sss"  -H "upstream-a2: a2"]])
    h.assert_contains(verbose,"< test: sss")
    h.assert_contains(verbose,"< test: 2")
    h.assert_contains(verbose,"< test: 3")

    h.assert_contains(verbose,"< a1: b1")

    h.assert_contains(verbose,"< a2: b1")
    h.assert_contains(verbose,"< a2: b2")