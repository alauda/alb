use strict;
use warnings;
use t::Alauda;
use Test::Nginx::Socket 'no_plan';

no_shuffle();
no_root_location();
run_tests();

__DATA__

=== TEST 1: http ping/pong should ok 
--- http_config
server {
    listen 9999;
    location /ping {
       content_by_lua_block {
    	    ngx.say("pong");
      }
    }
}
--- policy
{
  "certificate_map": {},
  "port_map": {
    "80": [
      {
        "rule": "test-rule-1",
        "dsl":"(STARTS_WITH URL /ping)",
        "internal_dsl": [
              "AND",
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
      "mode": "http",
      "backends": [
        {
          "address": "127.0.0.1",
          "port":9999,
          "weight": 100
        }
      ]
    }
  ]
}
--- server_port 
80
--- request
    GET /ping
--- response_body
pong

--- no_error_log
[error]
=== TEST 2: tcp ping/pong should ok
--- http_config
server {
    listen 9999;
    location /ping {
       content_by_lua_block {
    	    ngx.print("pong");
      }
    }
}
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
          "weight": 100
        }
      ]
    }
  ]
}
--- server_port: 81
--- request: GET /ping
--- response_body: pong
