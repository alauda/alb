-- format:on
local m = {}
local h = require "test-helper"
local u = require "util"
local p = require("policy_helper")
local F = require("F")
local utils = require("utils.common")
local http = require"resty.http".new()

function m.as_backend()
    local response = {header = ngx.req.get_headers(), url = ngx.var.request_uri}
    ngx.print(utils.json_encode(response))
end

function m.test()
    m.test_common()
    m.test_var()
end

function m.test_common()
    local policy = [[
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
              "rewrite_request": {
                  "headers": {
                      "a": "b",
                      "test": "xxxx"
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
              "rewrite_request": {
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
              "/add-header"
            ]
          ],
          "upstream": "test-upstream-1",
          "config": {
              "rewrite_request": {
                  "headers_add": {
                      "test": ["2","3"] ,
                      "a1": ["b1"] ,
                      "a2": ["b1","b2"]
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
              "/abc/1"
            ]
          ],
          "upstream": "test-upstream-2",
          "rewrite_prefix_match": "/abc/1",
          "rewrite_replace_prefix": "/dddd"
        },
        {
          "rule": "",
          "internal_dsl": [
            [
              "STARTS_WITH",
              "URL",
              "/abc/2"
            ]
          ],
          "upstream": "test-upstream-2",
          "rewrite_prefix_match": "/abc/",
          "rewrite_replace_prefix": ""
        }
        ]}},
  "backend_group": [
    {
      "name": "test-upstream-1",
      "mode": "http",
      "backends": [
        {
          "address": "127.0.0.1",
          "port": 1880,
          "weight": 100
        }
      ]
    },
    {
      "name": "test-upstream-2",
      "mode": "http",
      "backends": [
        {
          "address": "127.0.0.1",
          "port": 1880,
          "weight": 100
        }
      ]
    }
    ]
}
  ]]
    p.set_policy_json_str(policy)

    -- test rewrite_request headers echo-server
    do
        local out, err = http:request_uri("http://127.0.0.1:1880/xxxx", {headers = {test = "sss"}})
        local res = utils.json_decode(out.body)
        u.logs(res)
        h.assert_eq(res.url, "/xxxx")
        h.assert_eq(res.header.test, "sss")
    end

    -- test rewrite_request headers set
    do
        local out, err = http:request_uri("http://127.0.0.1:80/set-header", {headers = {test = "sss"}})
        local res = utils.json_decode(out.body)
        u.logs(res)
        h.assert_eq(res.header.test, "xxxx")
        h.assert_eq(res.header.a, "b")
    end
    --
    --
    do
        -- test rewrite_request headers remove
        local out, err = http:request_uri("http://127.0.0.1:80/remove-header", {headers = {test2 = "s", test1 = "sss"}})
        local res = utils.json_decode(out.body)
        h.assert_eq(res.header.test, nil)
        h.assert_eq(res.header.test1, nil)
        h.assert_eq(res.header.test2, "s")
    end

    -- test rewrite_request headers add
    do
        local out, err = http:request_uri("http://127.0.0.1:80/add-header", {headers = {["upstream-test"] = "sss", ["upstream-a2"] = "a2"}})
        local res = utils.json_decode(out.body)
        u.logs(res)
        u.logs(out.body)
        -- add header ok
        h.assert_eq(res.header.test, {"2", "3"})
        h.assert_eq(res.header.a1, "b1")
        h.assert_eq(res.header.a2, {"b1", "b2"})
    end
    --
    -- test rewrite_request url rewrite
    local out, err = http:request_uri("http://127.0.0.1:80/abc/1")
    local res = utils.json_decode(out.body)
    h.assert_eq(res.url, "/dddd")
    local out, err = http:request_uri("http://127.0.0.1:80/abc/2")
    local res = utils.json_decode(out.body)
    h.assert_eq(res.url, "/2")
    h.P(res)
end

function m.test_var()
    -- LuaFormatter off
    --
    local policy = [[
        {
          "http": {"tcp": {
              "80": [
                {
                  "rule": "",
                  "internal_dsl": [
                    [
                      "STARTS_WITH",
                      "URL",
                      "/t1"
                    ]
                  ],
                  "upstream": "test-upstream-1",
                  "config": {
                      "rewrite_request": {
                          "headers_var": {
                              "a-set": "cookie_a"
                          },
                          "headers_add_var": {
                              "a-add": ["cookie_a"],
                              "x-add": ["cookie_a","cookie_c"]
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
                  "port": 1880,
                  "weight": 100
                }
              ]
            }
          ]
        }
    ]]
    -- LuaFormatter on
    p.set_policy_json_str(policy)

    -- test header_var
    do
        local out, err = http:request_uri("http://127.0.0.1:80/t1", {headers = {["Cookie"] = "a=a1;c=c1"}})
        local res = utils.json_decode(out.body)
        u.logs(res, err)
        h.assert_eq(res.header["a-set"], "a1")
        h.assert_eq(res.header["a-add"], "a1")
    end
    do
        local out, err = http:request_uri("http://127.0.0.1:80/t1", {headers = {["Cookie"] = "a=1111;c=c1", ["a-add"] = "2222"}})
        local res = utils.json_decode(out.body)
        u.logs(res, err)
        h.assert_eq(res.header["a-set"], "1111")
        h.assert_eq(res.header["a-add"], {"2222", "1111"})
        h.assert_eq(res.header["x-add"], {"1111", "c1"})
    end
    -- not has cookie
    do
        local out, err = http:request_uri("http://127.0.0.1:80/t1", {headers = {["Cookie"] = "c=c1", ["a-add"] = "2222"}})
        local res = utils.json_decode(out.body)
        u.logs(res, err)
        h.assert_eq(res.header["a-set"], nil)
        h.assert_eq(res.header["a-add"], "2222")
        h.assert_eq(res.header["x-add"], "c1")
    end
end

return m
