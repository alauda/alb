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
  "port_map": {
    "80": [
      {
        "rule": "test-rule-1",
        "dsl":"(STARTS_WITH URL /ping)",
        "internal_dsl": [
              [
                "STARTS_WITH",
                "URL",
                "/ping"
              ]
            ],
        "upstream": "test-upstream-1",
        "url": "",
        "rewrite_base": "",
        "rewrite_target": "",
        "subsystem": "http",
        "enable_cors": false,
        "cors_allow_headers": "",
        "cors_allow_origin": "",
        "backend_protocol": "",
        "redirect_url": "",
        "redirect_code": 0,
        "vhost": ""
      }
    ]
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
  "port_map": {
    "81": [
      {
        "rule": "",
        "dsl":"",
        "upstream": "test-upstream-1",
        "url": "",
        "internal_dsl":"",
        "rewrite_base": "",
        "rewrite_target": "",
        "subsystem": "stream",
        "enable_cors": false,
        "cors_allow_headers": "",
        "cors_allow_origin": "",
        "backend_protocol": "",
        "redirect_url": "",
        "redirect_code": 0,
        "vhost": ""
      }
    ]
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