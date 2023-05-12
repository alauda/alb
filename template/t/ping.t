use strict;
use warnings;
use t::Alauda;
use Test::Nginx::Socket 'no_plan';
use Test::Nginx::Socket;


our $policy = <<'_EOC_';
{
  "certificate_map": {},
  "http": {"tcp": {"80": [
        {
          "rule": "",
          "internal_dsl": [
            [
              "STARTS_WITH",
              "URL",
              "/t1"
            ]
          ],
          "upstream": "test-upstream-1"
        }] }
  },
  "stream":{"tcp":{"81":[{"upstream":"test-upstream-1"}]}},
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
    }
  ]
}
_EOC_

our $http_config = <<'_EOC_';
server {
    listen 9999;
    location / {
       content_by_lua_block {
    	   ngx.print("ok");
      }
    }
}
_EOC_

log_level("info");
no_shuffle();
no_root_location();
run_tests(); 



__DATA__

=== TEST 1: http ping/pong should ok 
--- policy eval: $::policy
--- http_config eval: $::http_config
--- lua_test
	local F = require("F");local u = require("util");local h = require("test-helper");local httpc = require("resty.http").new();
    local res, err = httpc:request_uri("http://127.0.0.1:80/t1")
	u.log(F"{err}")
	u.log(F"test res  {res.body} err {err}")
	h.assert_curl_success(res,err,"ok")
--- response_body: ok

=== TEST 2: tcp ping/pong should ok 
--- policy eval: $::policy
--- http_config eval: $::http_config
--- lua_test
	local F = require("F");local u = require("util");local h = require("test-helper");
    local httpc = require("resty.http").new()

    local res, err = httpc:request_uri("http://127.0.0.1:81/t1")
	u.log(F"test res  {res.body} err {err}")
	h.assert_curl_success(res,err,"ok")
--- response_body: ok