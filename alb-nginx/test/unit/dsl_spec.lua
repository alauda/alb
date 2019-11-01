---
--- Created by oilbeater.
--- DateTime: 17/11/23 下午3:34
---

local dsl = require("dsl")

describe("DSL unit test", function()
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

        local result, err = dsl.eval(dsl.generate_ast("(AND (OR (EQ HOST baidu.com) (EQ HOST www.baidu.com)) (STARTS_WITH URL /search) (EQ SRC_IP 2.2.2.2) (RANGE HEADER UID 10 1000))"))
        assert.True(result)
        assert.Nil(err)
    end)

    it("dsl short circuit", function ()
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
        local result, err = dsl.eval(dsl.generate_ast("(AND (EQ HOST www.alauda.cn) (XXX adfaf adfaf))"))
        assert.False(result)
        assert.Nil(err)
        result, err = dsl.eval(dsl.generate_ast("(OR (EQ HOST www.baidu.com) (XXX adfaf adfaf))"))
        assert.True(result)
        assert.Nil(err)
    end)
end)
