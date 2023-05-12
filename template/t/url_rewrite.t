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
              "/es_proxy"
            ]
          ],
		  "rewrite_target":"/$2",
		  "rewrite_base":"/es_proxy(/|$)(.*)",
          "upstream": "test-upstream-1"
        }] }
  },
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
    	   ngx.print(ngx.var.request_uri);
      }
    }
}
_EOC_

log_level("info");
no_shuffle();
no_root_location();
run_tests(); 

__DATA__

=== TEST 1: http url rewrite 
--- policy eval: $::policy
--- http_config eval: $::http_config
--- lua_test
	local F = require("F");local u = require("util");local h = require("test-helper");
    local httpc = require("resty.http").new()
    local res, err = httpc:request_uri("http://127.0.0.1:80/es_proxy/v1")
	h.assert_eq(res.body, "/v1")