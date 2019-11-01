---
--- Created by oilbeater.
--- DateTime: 17/11/16 下午4:22
---
local op = require("operation")
local cjson = require("cjson")
local helper = require("test.helper")

describe("operation unit test", function()
    it("normal condition test", function()
        ngx.var = {
            host = "www.baidu.com",
            uri  = "/search",
            remote_addr = "2.2.2.2",
            http_test_header = "test-header",
            http_UID = "100",
            http_UUID = "x",
            cookie_test_cookie = "test-cookie"
        }

        ngx.req = {
            get_uri_args = function() return {UID = "100"} end
        }
        local test_table = {
            {"AND", {true, false}, false, nil},
            {"AND", {true, true}, true, nil},
            {"AND", {false, true}, false, nil},
            {"OR", {true, false}, true, nil},
            {"OR", {true, true}, true, nil},
            {"OR", {false, false}, false, nil},
            {"EQ", {"HOST", "www.baidu.com"}, true, nil},
            {"EQ", {"HOST", "baidu.com"}, false, nil},
            {"EQ", {"SRC_IP", "2.2.2.2"}, true, nil},
            {"EQ", {"SRC_IP", "1.1.1.1"}, false, nil},
            {"IN", {"HOST", "www.baidu.com", "www.sina.com.cn"}, true, nil},
            {"IN", {"HOST", "www.bdu.com", "www.sina.com.cn"}, false, nil},
            {"RANGE", {"SRC_IP", "1.99.99.99", "3.3.3.3"}, true, nil},
            {"RANGE", {"SRC_IP", "2.2.2.3", "3.3.3.3"}, false, nil},
            {"RANGE", {"HEADER", "UID", "1", "10000"}, true, nil},
            {"RANGE", {"HEADER", "UID", "1", "10"}, false, nil},
            {"RANGE", {"HEADER", "UUID", "a", "z"}, true, nil},
            {"RANGE", {"HEADER", "UUID", "a", "c"}, false, nil},
            {"EXIST", {"PARAM", "UID"}, true, nil},
            {"EXIST", {"PARAM", "UUID"}, false, nil},
            {"EXIST", {"COOKIE", "test_cookie"}, true, nil},
            {"STARTS_WITH", {"URL", "/search"}, true, nil},
            {"STARTS_WITH", {"URL", "/searchh"}, false, nil},
            {"REGEX", {"URL", "/s[a-z]*"}, true, nil},
            {"REGEX", {"URL", "/b[a-z]*"}, false, nil}
        }

        for _, v in ipairs(test_table) do
            local origin = helper.deepcopy(v)
            local result, err = op.eval(v[1], v[2])
            assert((result == v[3]), cjson.encode(origin))
            assert((err == v[4]), cjson.encode(origin))
        end
    end)

    it("matcher not exists condition test", function ()
        ngx.var = {}
        ngx.req = {
            get_uri_args = function() return {} end
        }
        local test_table = {
            {"EQ", {"HOST", "www.baidu.com"}, false, nil},
            {"EQ", {"SRC_IP", "1.1.1.1"}, false, nil},
            {"RANGE", {"HEADER", "UID", "1", "10000"}, false, nil},
            {"EXIST", {"COOKIE", "test_cookie"}, false, nil},
            {"STARTS_WITH", {"URL", "/search"}, false, nil},
            {"REGEX", {"URL", "/b[a-z]*"}, false, nil}
        }

        for _, v in ipairs(test_table) do
            local origin = helper.deepcopy(v)
            local result, _ = op.eval(v[1], v[2])
            assert((result == v[3]), cjson.encode(origin))
        end
    end)
end)