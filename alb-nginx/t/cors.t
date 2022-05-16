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
              "/t"
            ]
          ],
		  "enable_cors": true,
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

log_level("info");
no_shuffle();
no_root_location();
run_tests(); 

__DATA__

=== TEST 1: http cors 
--- policy eval: $::policy
--- lua_test
	local F = require("F");local u = require("util");local h = require("test-helper");
    local httpc = require("resty.http").new()
    local res, err = httpc:request_uri("http://127.0.0.1:80/t",{method="OPTIONS"})
	u.log(F"{u.inspect(res)}")
	h.assert_eq(res.status,204)
	h.assert_eq(res.headers["Access-Control-Allow-Origin"],"*")