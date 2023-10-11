use strict;
use warnings;
use t::Alauda;
use Test::Nginx::Socket 'no_plan';

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
          "upstream": "test-upstream-1"
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
--- lua_test_eval: require('e2e.retry.retry').test()
