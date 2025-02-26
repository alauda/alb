use strict;
use warnings;
use t::Alauda;
use Test::Nginx::Socket 'no_plan';
our $tt = t::Alauda::get_test_name(__FILE__);

log_level('warn');
no_shuffle();
no_root_location();

run_tests();

__DATA__

=== TEST 1: http and tcp should only retry five time
--- policy
{
  "certificate_map": {},
  "stream": {
    "tcp": {
      "81": [
        {
          "upstream": "test-upstream-1",
          "plugins":["timeout"],
          "config": {
            "timeout": {
            "proxy_connect_timeout_ms":1000
            }
          }
        }
      ]
    }
  },
  "http": {
    "tcp":{
        "80": [
            {
              "rule": "test-rule-1",
              "internal_dsl": [["STARTS_WITH","URL","/ping"]],
              "upstream": "test-upstream-1"
            }
        ]
    }
  },
  "backend_group": [
    {
      "name": "test-upstream-1",
      "mode": "http",
      "backends": [
        {
          "address": "127.0.0.1",
          "port":9999,
          "weight": 25
        },
        {
          "address": "127.0.0.1",
          "port":9999,
          "weight": 25
        },
        {
          "address": "127.0.0.1",
          "port":9999,
          "weight": 25
        },
        {
          "address": "127.0.0.1",
          "port":9999,
          "weight": 25
        }
      ]
    }
  ]
}
--- lua_test_eval eval: "require('$::tt').test()"
