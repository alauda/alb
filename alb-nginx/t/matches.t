use strict;
use warnings;
use t::Alauda;
use Test::Nginx::Socket 'no_plan';
use Test::Nginx::Socket;

no_shuffle();
no_root_location();
run_tests();

__DATA__
=== TEST 1: should match request
--- log_level: info
--- config
location /t {
    content_by_lua_block {
      package.path = '/t/lib/?.lua;' .. package.path;
      local util = require("util")
      local test = require("test-helper");

      -- should not match without header
      local res, err = util.curl("http://127.0.0.1:80/v1",{})
      test.assert_eq(res.status,404)

      local res, err = util.curl("http://127.0.0.1:80/v1",{ 
        headers = {
          ["host"] = "a.com",
          ["version"] = "1.1"
        }
      })
      test.assert_curl_success(res,err)

      local res, err = util.curl("http://127.0.0.1:80/v1",{ 
        headers = {
          ["host"] = "a.b.com",
          ["version"] = "1.1"
        }
      })
      test.assert_curl_success(res,err)

      local res, err = util.curl("http://127.0.0.1:80/v1",{ 
        headers = {
          ["host"] = "c.com",
          ["version"] = "1.1"
        }
      })
      test.assert_eq(res.status,404)

      local res, err = util.curl("http://127.0.0.1:80/v1",{ 
        headers = {
          ["host"] = "a.com",
          ["version"] = "2.1"
        }
      })
      test.assert_eq(res.status,404)
      ngx.say("success")
    }
}

--- policy
{
  "certificate_map": {},
  "http": {"tcp":{"80": 
    [
      {
        "rule": "test-rule-1",
        "internal_dsl": ["AND",["IN","HOST","a.com","a.b.com"],["STARTS_WITH","URL","/v1"],["EQ","HEADER","version","1.1"]],
        "upstream": "test-upstream-1"
      }
    ]}},
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
--- http_config
server {
    listen 9999;
    location / {
        content_by_lua_block {
          ngx.print("success");
      }
    }
}
--- request
    GET /t
--- response_body
success

=== TEST 2: should match request method
--- log_level: info
--- http_config
server {
    listen 9999;
    location /t1 {
        content_by_lua_block {
          ngx.print("t1");
      }
    }
    location /t2 {
        content_by_lua_block {
          ngx.print("t2");
      }
    }
}
--- policy
{
  "http": {"tcp": {"80": [
        {
          "rule": "",
          "internal_dsl": [
            ["AND",["EQ","METHOD","GET"],["STARTS_WITH","URL","/t1"]]
          ],
          "upstream": "test-upstream-1"
        },
        {
          "rule": "",
          "internal_dsl": [
            ["AND",["EQ","METHOD","POST"],["STARTS_WITH","URL","/t2"]]
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
          "port": 9999,
          "weight": 100
        }
      ]
    }
  ]
}

--- lua_test
	local F = require("F");local u = require("util");local h = require("test-helper");
    local httpc = require("resty.http").new()
	do
    	local res, err = httpc:request_uri("http://127.0.0.1:80/t1")
		h.assert_curl_success(res,err,"t1")
	end

	do 
    	local res, err = httpc:request_uri("http://127.0.0.1:80/t2",{
		    	method = "PUT",
		})
		h.assert_eq(res.status,404)
	end

	do 
    	local res, err = httpc:request_uri("http://127.0.0.1:80/t2",{
		    	method = "POST",
		})
		h.assert_curl_success(res,err,"t2")
	end

--- response_body: ok