use strict;
use warnings;
use t::Alauda;
use Test::Nginx::Socket 'no_plan';


our $backends = qq{
      "backends": [
        {
          "address": "127.0.0.1",
          "port":9000,
          "weight": 20
        },
        {
          "address": "127.0.0.1",
          "port":9001,
          "weight": 20
        },
        {
          "address": "127.0.0.1",
          "port":9002,
          "weight": 20
        },
        {
          "address": "127.0.0.1",
          "port":9003,
          "weight": 20
        },
        {
          "address": "127.0.0.1",
          "port":9004,
          "weight": 20
        }
      ]
};

our $HttpConfig = qq{
server {
    listen 9000;
    location / {
       content_by_lua_block {
    	    ngx.say("9000");
      }
    }
}

server {
    listen 9001;
    location / {
       content_by_lua_block {
    	    ngx.say("9001");
      }
    }
}

server {
    listen 9002;
    location / {
       content_by_lua_block {
    	    ngx.say("9002");
      }
    }
}

server {
    listen 9003;
    location / {
       content_by_lua_block {
    	    ngx.say("9003");
      }
    }
}

server {
    listen 9004;
    location / {
       content_by_lua_block {
    	    ngx.say("9004");
      }
    }
}
};


no_shuffle();
no_root_location();
run_tests();
# 1. sticky cookie should work
# 2. should set cookie
# 3. stick header should work
__DATA__

=== TEST 1: roundrobin should work
--- http_config eval: $::HttpConfig
--- config
    location /t {
      content_by_lua_block {
          local test = require("balancer_test");
          if test.test_balancer("roundrobin") then 
            ngx.print("success");
          else
            ngx.print("fail");
          end
      }
    }
--- policy eval
qq|
{
  "certificate_map": {},
  "http":{"tcp":{"80":[{
    "rule":"test-rule-1",
    "internal_dsl":[["STARTS_WITH","URL","/"]],
    "upstream":"test-upstream-1"}
  ]}},
  "backend_group": [
    {
      "name": "test-upstream-1",
      "mode": "http",
      "session_affinity_policy": "",
      "session_affinity_attribute": "",
      $::backends
    }
  ]
}
|;

--- curl
--- http2
--- request
GET /ping
--- request: GET /t
--- response_body: success

=== TEST 2: sticky header should work
--- http_config eval: $::HttpConfig
--- config
    location /t {
      content_by_lua_block {
          package.path = '/t/?.lua;/t/lib/?.lua;' .. package.path;
          local test = require("balancer_test");
           if test.test_balancer("sticky_header","h1") then
             ngx.print("success");
           else
             ngx.print("fail");
           end
      }
    }
--- policy eval
qq|
{
  "certificate_map": {},
  "http":{"tcp":{"80":[{
    "rule":"test-rule-1",
    "internal_dsl":[["STARTS_WITH","URL","/"]],
    "upstream":"test-upstream-1"}
  ]}},
  "backend_group": [
    {
      "name": "test-upstream-1",
      "mode": "http",
      "session_affinity_policy": "header",
      "session_affinity_attribute": "h1",
      $::backends
    }
  ]
}
|;

--- request
  GET /t
--- response_body: success


=== TEST 3: sticky cookie should work
--- http_config eval: $::HttpConfig
--- config
    location /t {
      content_by_lua_block {
          package.path = '/t/?.lua;/t/lib/?.lua;' .. package.path;
          local test = require("balancer_test");
           if test.test_balancer("sticky_cookie","h1") then
             ngx.print("success");
           else
             ngx.print("fail");
           end
      }
    }
--- policy eval
qq|
{
  "certificate_map": {},
  "http":{"tcp":{"80":[{
    "rule":"test-rule-1",
    "internal_dsl":[["STARTS_WITH","URL","/"]],
    "upstream":"test-upstream-1"}
  ]}},
  "backend_group": [
    {
      "name": "test-upstream-1",
      "mode": "http",
      "session_affinity_policy": "cookie",
      "session_affinity_attribute": "h1",
      $::backends
    }
  ]
}
|;

--- request
  GET /t
--- response_body: success

=== TEST 4: default sticky cookie session_affinity_attribute is JSESSIONID 
--- http_config eval: $::HttpConfig
--- config
    location /t {
      content_by_lua_block {
          package.path = '/t/?.lua;/t/lib/?.lua;' .. package.path;
          local test = require("balancer_test");
           if test.test_balancer("sticky_cookie","") then
             ngx.print("success");
           else
             ngx.print("fail");
           end
      }
    }
--- policy eval
qq|
{
  "certificate_map": {},
  "http":{"tcp":{"80":[{
    "rule":"test-rule-1",
    "internal_dsl":[["STARTS_WITH","URL","/"]],
    "upstream":"test-upstream-1"}
  ]}},
  "backend_group": [
    {
      "name": "test-upstream-1",
      "mode": "http",
      "session_affinity_policy": "cookie",
      "session_affinity_attribute": "",
      $::backends
    }
  ]
}
|;

--- request
  GET /t
--- response_body
success