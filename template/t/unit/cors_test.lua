local _M = {}

local h = require("test-helper");
local cors = require("cors")

function _M.test()
    h.assert_true(cors.origin_contains("http://a.com,http://b.com", "http://a.com"))
    h.assert_true(cors.origin_contains("http://a.com,http://b.com", "http://b.com"))
    h.assert_true(cors.origin_contains("http://a.com,http://b.com:123", "http://b.com:123"))
    h.assert_true(not cors.origin_contains("http://a.com:80,http://b.com:123", "http://a.com"))
    h.assert_true(not cors.origin_contains("http://a.com.cn,http://b.com", "http://a.com"))
end

return _M
