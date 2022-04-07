use strict;
use warnings;
use t::Alauda;
use Test::Nginx::Socket 'no_plan';
use Test::Nginx::Socket;

our $policy = <<'_EOC_';
{
  "certificate_map": {},
  "http": {"tcp": {
      "80": [
        {
          "rule": "",
          "internal_dsl": [
            [
              "STARTS_WITH",
              "URL",
              "/set-header"
            ]
          ],
          "upstream": "test-upstream-1",
          "config": {
              "rewrite_response": {
                  "headers": {
                      "a": "b"
                  }
              }
          }
        },
        {
          "rule": "",
          "internal_dsl": [
            [
              "STARTS_WITH",
              "URL",
              "/remove-header"
            ]
          ],
          "upstream": "test-upstream-1",
          "config": {
              "rewrite_response": {
                  "headers_remove": [
                      "test","test1"
                  ]
              }
          }
        },
        {
          "rule": "",
          "internal_dsl": [
            [
              "STARTS_WITH",
              "URL",
              "/default-header"
            ]
          ],
          "upstream": "test-upstream-1",
          "config": {
              "rewrite_response": {
                  "headers_default": {
                      "test": "test1"
                  }
              }
          }
        },
        {
          "rule": "",
          "internal_dsl": [
            [
              "STARTS_WITH",
              "URL",
              "/update-header"
            ]
          ],
          "upstream": "test-upstream-1",
          "config": {
              "rewrite_response": {
                  "headers_update": {
                      "test": "test1"
                  }
              }
          }
        },
        {
          "rule": "",
          "internal_dsl": [
            [
              "STARTS_WITH",
              "URL",
              "/add-header"
            ]
          ],
          "upstream": "test-upstream-1",
          "config": {
              "rewrite_response": {
                  "headers_add": {
                      "test": ["2","3"]
                  }
              }
          }
        }
        ]}},
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
    }]
}
_EOC_
our $http_config = <<'_EOC_';
server {
    listen 9999;
    location / {
       content_by_lua_block {
           local h, err = ngx.req.get_headers()
           for k, v in pairs(h) do
               ngx.header[k] = v
           end
           ngx.say("ok")
      }
    }
}
_EOC_


log_level("info");
no_shuffle();
no_root_location();
run_tests();


__DATA__

=== TEST 1: test header set 
--- ONLY
--- policy eval: $::policy
--- http_config eval: $::http_config
--- lua_test
    local F = require("F");local u = require("util");local h = require("test-helper");
    local httpc = require("resty.http").new()
    local res, err = httpc:request_uri("http://127.0.0.1:9999/t",{
        headers = {
            ["test"] = "sss"
        }
    })
    h.assert_eq(res.headers.test,"sss")

    -- set header ok
    local res, err = httpc:request_uri("http://127.0.0.1:80/set-header",{
        headers = {
            ["test"] = "sss"
        }
    })
    h.assert_eq(res.headers.test,"sss")
    h.assert_eq(res.headers.a,"b")

    -- remove header ok
    local res, err = httpc:request_uri("http://127.0.0.1:80/remove-header",{
        headers = {
            ["test"] = "sss",
            ["test1"] = "sss",
            ["a"] = "b"
        }
    })
    h.assert_eq(res.headers.test,nil)
    h.assert_eq(res.headers.test1,nil)
    h.assert_eq(res.headers.a,"b")

    -- default header ok
    local res, err = httpc:request_uri("http://127.0.0.1:80/default-header",{
        headers = {
            ["test"] = "xxxxx",
        }
    })
    h.assert_eq(res.headers.test,"xxxxx")
    local res, err = httpc:request_uri("http://127.0.0.1:80/default-header",{
        headers = {
        }
    })
    h.assert_eq(res.headers.test,"test1")

    -- update header ok
    local res, err = httpc:request_uri("http://127.0.0.1:80/update-header",{
        headers = {
            ["test"] = "xxxxx",
        }
    })
    h.assert_eq(res.headers.test,"test1")
    local res, err = httpc:request_uri("http://127.0.0.1:80/update-header",{
        headers = {
        }
    })
    h.assert_eq(res.headers.test,nil)

    -- add header ok
    local res, err = httpc:request_uri("http://127.0.0.1:80/add-header",{
        headers = {
            ["test"] = "1",
        }
    })
    h.assert_eq(res.headers.test,"1,2,3")

--- response_body: ok
