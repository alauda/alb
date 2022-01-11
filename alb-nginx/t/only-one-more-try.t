use strict;
use warnings;
use t::Alauda;
use Test::Nginx::Socket 'no_plan';

log_level('warn');
no_shuffle();
no_root_location();

run_tests();

__DATA__

=== TEST 1: http should only retry five time
--- policy
{
  "certificate_map": {},
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
--- server_port: 80
--- request
    GET /ping
--- access_log
502 502, 502, 502, 502, 502 127.0.0.1
--- error_code: 502

=== TEST 2: tcp should only retry 5 times
--- ignore_response
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
  "backend_group": [
    {
      "name": "test-upstream-1",
      "session_affinity_policy": "",
      "session_affinity_attribute": "",
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
--- server_port: 81
--- request
    GET /ping
--- grep_error_log eval
qr/Connection refused/
--- grep_error_log_out
Connection refused
Connection refused
Connection refused
Connection refused
Connection refused
