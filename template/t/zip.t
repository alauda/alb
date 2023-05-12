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
          "internal_dsl": [["STARTS_WITH","URL","/t1"]],
          "upstream": "test-upstream-1"
        }
    ]}
  },
  "stream":{"tcp":{"81":[{"upstream":"test-upstream-1"}]}},
  "backend_group": [ { "name": "test-upstream-1", "mode": "http", "backends": [{"address":"127.0.0.1", "port": 9999, "weight":100}]}]
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
=== TEST 1: test zlib
--- policy eval: $::policy
--- http_config eval: $::http_config
--- lua_test
    local zlib = require('lua-ffi-zlib.lib.ffi-zlib')
	local F = require("F");local u = require("util");local h = require("test-helper");
    u.log("zlib.version() "..zlib.version())
    h.assert_eq(zlib.version()~="",true)

=== TEST 2: test decompress_from_file
--- ONLY
--- policy eval: $::policy
--- http_config eval: $::http_config
--- lua_test
    local zlib = require('lua-ffi-zlib.lib.ffi-zlib')
	local F = require("F");local u = require("util");local t = require("test-helper");

    local base=os.getenv("TEST_BASE")
    local compress = require('utils.compress')
    local p = base.."/t/resource/policy.bin"
    u.log("p "..p)
    local out,err = compress.decompress_from_file(p)
    local expect = [[
{
  "http": {
    "tcp": {
      "81": {
        "rule": "rule2",
        "upstream": "u2"
      }
    }
  },
  "backend_group": [
    {
      "name": "u2",
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
]]

    ngx.log(ngx.INFO, "out "..tostring(out))
    ngx.log(ngx.INFO, "err "..tostring(err))
    if err ~= nil then
        ngx.log(ngx.ERR, "err not nil "..err)
        ngx.exit(ngx.ERROR,"err not nil")
    end
    t.assert_eq(err,nil)
    t.assert_eq(t.trim(expect),t.trim(out))

    -- file not exist
    local out,err = compress.decompress_from_file(base.."/t/resource/policy.bin.notexist")
    ngx.log(ngx.INFO, "out "..tostring(out))
    ngx.log(ngx.INFO, "err "..tostring(err))
    t.assert_contains(err,"file is nil")

    -- empty file
    local out,err = compress.decompress_from_file(base.."/t/resource/policy.bin.empty")
    ngx.log(ngx.INFO, "out "..tostring(out))
    ngx.log(ngx.INFO, "err "..tostring(err))
    t.assert_contains(err,"no input bytes")

    -- invalid format
    local out,err = compress.decompress_from_file(base.."/t/resource/policy.bin.invalid")
    ngx.log(ngx.INFO, "out "..tostring(out))
    ngx.log(ngx.INFO, "err "..tostring(err))
    t.assert_contains(err,"data error")
