use strict;
use warnings;
use t::Alauda;
use Test::Nginx::Socket 'no_plan';
use Test::Nginx::Socket;

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
  "stream":{"tcp":{"81":[{"upstream":"test-upstream-1"}]}},
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
