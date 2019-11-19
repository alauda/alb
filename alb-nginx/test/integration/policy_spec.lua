---
--- Created by oilbeater.
--- DateTime: 17/11/30 下午2:45
---

local policy_endpoint = "http://127.0.0.1:1936/policies"
local service_endpoint = "http://127.0.0.1:8080"

local http = require("resty.http")
local cjson = require("cjson")
local httpc = http.new()

describe("check policy run as excepted", function ()
    it("no policy", function ()
        httpc:request_uri(policy_endpoint, {
            method = "PUT",
            body   = "{}"
        })
        local res, err = httpc:request_uri(policy_endpoint, {
            method = "GET"
        })
        assert.Nil(err)
        assert.are.equals(200, res.status)
        assert.are.equals("{}", res.body)

        res, err = httpc:request_uri(service_endpoint, {
            method = "GET"
        })
        assert.Nil(err)
        assert.are.equals(404, res.status)
    end)

    it("set policy and get", function ()
        local policy = [[{
                        "8080": [
                            {
                                "rule": "(AND (STARTS_WITH URL /search) (RANGE HEADER uid 10 100))",
                                "upstream": "v1"
                            },
                            {
                                "rule": "(AND (RANGE SRC_IP 0.0.0.0 255.255.255.255) (EQ HOST www.baidu.com))",
                                "upstream": "v2"
                            }
                        ]
                    }]]
        local res, err = httpc:request_uri(policy_endpoint, {
            method = "PUT",
            body   = policy
        })
        assert.Nil(err)
        assert.are.equals(200, res.status)
        assert.are.equals(cjson.encode(cjson.decode(policy)), cjson.encode(cjson.decode(res.body)))
    end)

    it("check policy take effect", function()
        local res, err = httpc:request_uri(service_endpoint.."/search?xxx=ddd", {
            method = "GET",
            headers = {
                uid = 55
            }
        })
        assert.Nil(err)
        assert.are.equals(200, res.status)
        assert.are.equals("v1", cjson.decode(res.body)["version"])

        res, err = httpc:request_uri(service_endpoint.."/search?xxx=ddd", {
            method = "GET",
            headers = {
                host = "www.baidu.com"
            }
        })
        assert.Nil(err)
        assert.are.equals(200, res.status)
        assert.are.equals("v2", cjson.decode(res.body)["version"])

        res, err = httpc:request_uri(service_endpoint.."/search?xxx=ddd", {
            method = "GET"
        })
        assert.Nil(err)
        assert.are.equals(404, res.status)

    end)
end)