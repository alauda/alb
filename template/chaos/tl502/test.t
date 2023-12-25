use strict;
use warnings;
use t::Alauda;
use Test::Nginx::Socket 'no_plan';
use Test::Nginx::Socket;

# rewrite_response could not set Connection header ..
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
              "/rewrite"
            ]
          ],
          "config": {
              "rewrite_response": {
                  "headers": {
                      "Connection": "close"
                  }
              }
          },
          "upstream": "test-upstream-1"
        },
        {
          "rule": "",
          "internal_dsl": [
            [
              "STARTS_WITH",
              "URL",
              "/"
            ]
          ],
          "upstream": "test-upstream-1"
        }
        ] }
  },
  "backend_group": [
    {
      "name": "test-upstream-1",
      "mode": "http",
      "backends": [
        {
          "address": "127.0.0.1",
          "port": 65432,
          "weight": 100
        }
      ]
    }
  ]
}
_EOC_

our $http_config = <<'_EOC_';
_EOC_

log_level("info");
no_shuffle();
no_root_location();
run_tests(); 



__DATA__

=== TEST 1: http ping/pong should ok 
--- policy eval: $::policy
--- http_config eval: $::http_config
--- timeout: 9999999
--- lua_test_eval: require("chaos.tl502.test").test()
