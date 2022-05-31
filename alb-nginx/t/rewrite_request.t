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
              "rewrite_request": {
                  "headers": {
                      "a": "b",
                      "test": "xxxx"
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
              "rewrite_request": {
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
              "rewrite_request": {
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
           ngx.print(ngx.req.raw_header())
      }
    }
}
_EOC_


log_level("info");
no_shuffle();
no_root_location();
run_tests();


__DATA__

=== TEST 1: test rewrite_request headers echo-server
--- policy eval: $::policy
--- http_config eval: $::http_config
--- lua_test
    local F = require("F");local u = require("util");local h = require("test-helper");
    local out, verbose = u.shell_curl([[ curl -v -s http://127.0.0.1:9999/t -H "test: sss"]])
	u.log(F"out--\n{out}\nverbose\n{verbose}\n")
    h.assert_contains(verbose,"test: sss")

=== TEST 2: test rewrite_request headers set 
--- policy eval: $::policy
--- http_config eval: $::http_config
--- lua_test
    local F = require("F");local u = require("util");local h = require("test-helper");
    local out, verbose = u.shell_curl([[ curl -v -s http://127.0.0.1:80/set-header -H "test: sss"]])
	u.log(F"out--\n{out}\nverbose\n{verbose}\n")
    h.assert_contains(out,"test: xxxx")
    h.assert_contains(out,"a: b")


=== TEST 2: test rewrite_request headers remove
--- policy eval: $::policy
--- http_config eval: $::http_config
--- lua_test
    local F = require("F");local u = require("util");local h = require("test-helper");
    local out, verbose = u.shell_curl([[ curl -v -s http://127.0.0.1:80/remove-header -H "test2: s" -H "test1: s" -H "test: s"]])
    h.assert_not_contains(out,"test: s")
    h.assert_not_contains(out,"test1: s")
    h.assert_contains(out,"test2: s")

=== TEST 3: test rewrite_request headers add
--- policy eval: $::policy
--- http_config eval: $::http_config
--- lua_test
    local F = require("F");local u = require("util");local h = require("test-helper");
    -- add header ok
    local out, verbose = u.shell_curl([[ curl -v -s http://127.0.0.1:80/add-header -H "upstream-test: sss"  -H "upstream-a2: a2"]])
    h.assert_contains(out,"test: sss")
    h.assert_contains(out,"test: 2")
    h.assert_contains(out,"test: 3")
    h.assert_contains(out,"a1: b1")

    h.assert_contains(out,"a2: b1")
    h.assert_contains(out,"a2: b2")