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
          "internal_dsl": [["STARTS_WITH", "URL", "/t1"]],
		  "redirect_url": "/t1-1",
		  "redirect_code": 302,
          "upstream": "test-upstream-1"
        },
        {
          "rule": "",
          "internal_dsl": [["STARTS_WITH", "URL", "/t2"]],
		  "redirect_url": "/t2-1",
		  "redirect_code": 301,
          "upstream": "test-upstream-1"
        },
        {
          "rule": "",
          "internal_dsl": [["STARTS_WITH", "URL", "/t3"]],
		  "redirect_url": "/t3-1",
		  "redirect_scheme": "https",
		  "redirect_host": "a.com",
		  "redirect_port": 9090,
		  "redirect_code": 308,
          "upstream": "test-upstream-1"
        },
        {
          "rule": "",
          "internal_dsl": [["STARTS_WITH", "URL", "/t4"]],
		  "redirect_url": "/t4-1",
		  "redirect_host": "a.com",
          "upstream": "test-upstream-1"
        },
        {
          "rule": "",
          "internal_dsl": [["STARTS_WITH", "URL", "/t5"]],
		  "redirect_scheme": "https",
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
_EOC_

our $http_config = <<'_EOC_';
server {
    listen 9999;
    location / {
       content_by_lua_block {
    	   ngx.print("ok");
      }
    }
}
_EOC_

log_level("info");
no_shuffle();
no_root_location();
run_tests(); 

__DATA__

=== TEST 1: basic redirect should ok
--- policy eval: $::policy
--- http_config eval: $::http_config
--- lua_test
	local F = require("F");local u = require("util");local h = require("test-helper");
    local httpc = require("resty.http").new()

    local res, err = httpc:request_uri("http://127.0.0.1:80/t1")
	u.log(F"test res {res.status} {u.inspect(res.headers)} {res.body} err {err}")
	h.assert_eq(res.headers.Location,"/t1-1")
	h.assert_eq(res.status,302)

    local res, err = httpc:request_uri("http://127.0.0.1:80/t2")
	u.log(F"test res {res.status} {u.inspect(res.headers)} {res.body} err {err}")
	h.assert_eq(res.headers.Location,"/t2-1")
	h.assert_eq(res.status,301)
--- response_body: ok

=== TEST 2: redirect should ok
--- ONLY
--- policy eval: $::policy
--- http_config eval: $::http_config
--- lua_test
	local F = require("F");local u = require("util");local h = require("test-helper");
    local httpc = require("resty.http").new()

	do
    	local res, err = httpc:request_uri("http://127.0.0.1:80/t3")
		local status = res.status
		local location = res.headers['Location'] 
		h.assert_eq(status,308)
		h.assert_eq(location,"https://a.com:9090/t3-1")
	end

	do
    	local res, err = httpc:request_uri("http://127.0.0.1:80/t4")
		local status = res.status
		local location = res.headers['Location'] 
		h.assert_eq(status,302)
		h.assert_eq(location,"http://a.com/t4-1")
	end

	do
    	local res, err = httpc:request_uri("http://127.0.0.1:80/t5")
		local status = res.status
		local location = res.headers['Location'] 
		h.assert_eq(status,302)
		h.assert_eq(location,"https://127.0.0.1/t5")
	end

--- response_body: ok