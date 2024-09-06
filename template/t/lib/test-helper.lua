-- format:on
local _M = {}
local inspect = require "inspect"
local u = require "util"
local c = require "utils.common"
function _M.trim(s)
    return s:gsub("^%s*(.-)%s*$", "%1")
end

function _M.P(x)
    ngx.log(ngx.INFO, inspect.inspect(x))
end

-- test-helper should not use anything in common.lua,we just make a copy here.
function _M._table_equals(t1, t2, ignore_mt)
    local ty1 = type(t1)
    local ty2 = type(t2)
    if ty1 ~= ty2 then
        return false
    end
    -- non-table types can be directly compared
    if ty1 ~= "table" and ty2 ~= "table" then
        return t1 == t2
    end
    -- as well as tables which have the metamethod __eq
    local mt = getmetatable(t1)
    if not ignore_mt and mt and mt.__eq then
        return t1 == t2
    end
    for k1, v1 in pairs(t1) do
        local v2 = t2[k1]
        if v2 == nil or not _M._table_equals(v1, v2) then
            return false
        end
    end
    for k2, v2 in pairs(t2) do
        local v1 = t1[k2]
        if v1 == nil or not _M._table_equals(v1, v2) then
            return false
        end
    end
    return true
end

function _M.assert_table_equals(t1, t2, ignore_mt)
    if not _M._table_equals(t1, t2, ignore_mt) then
        ngx.log(ngx.ERR, "t1 != t2")
        _M.P(t1)
        _M.P(t2)
        _M.fail("t1 != t2")
    end
end

function _M.assert_contains(left, right)
    if left:find(right, 1, true) then
        return true
    end
    _M.fail("could not find " .. right .. " in " .. left)
end

function _M.assert_not_contains(left, right, msg)
    local find = left:find(right, 1, true)
    if find == nil then
        return true
    end
    _M.fail("could find " .. right .. " in " .. left .. " msg " .. tostring(msg))
end

function _M.assert_eq(left, right, msg)
    local ty1 = type(left)
    local ty2 = type(right)
    if ty1 ~= ty2 then
        _M.fail("type not same " .. ty1 .. ":" .. tostring(left) .. " " .. ty2 .. ":" .. tostring(right))
    end
    if ty1 == ty2 and ty1 == "table" then
        _M.assert_table_equals(left, right)
        return
    end
    if left ~= right then
        _M.fail(
            tostring(left) ..
            " ? " .. tostring(right) .. "  " .. tostring(left == right) .. " msg " .. tostring(msg))
        return
    end
end

function _M.assert_true(v, msg)
    _M.assert_eq(v, true, msg)
end

---comment
---@param v any # should be nil
---@param msg? string
function _M.assert_is_nil(v, msg)
    _M.assert_eq(v, nil, msg)
end

---comment
---@param v any # should be nil
---@param msg? string
function _M.assert_is_nil_or_empty_string(v, msg)
    if v == nil or v == "" then
        return
    end
    if msg == nil then
        msg = ""
    end
    _M.fail(tostring(v) .. " is not nil or empty " .. tostring(msg))
end

function _M.fail(msg)
    ngx.log(ngx.ERR, tostring(msg))
    ngx.log(ngx.ERR, u.get_caller_info(3, 5))
    ngx.exit(ngx.ERR)
end

function _M.just_curl(url, req_cfg)
    local res, err = u.curl(url, req_cfg)
    return res, err
end

--- curl and assert
---@param url string
---@param req_cfg table | nil
---@param assert_cfg table | nil
---@return table
function _M.assert_curl(url, req_cfg, assert_cfg)
    local this = _M
    local res, err = u.curl(url, req_cfg)
    this.assert_is_nil(err)
    if assert_cfg ~= nil and assert_cfg.status ~= nil then
        this.assert_eq(res.status, assert_cfg.status, _M.curl_res_to_string(res))
        return res
    end
    this.assert_eq(res.status, 200, _M.curl_res_to_string(res))
    return res
end

function _M.curl_res_to_string(res)
    local t = { body = res.body, headers = res.headers, status = res.status }
    return c.json_encode(t)
end

function _M.assert_curl_success(res, err, body)
    local res_body = _M.trim(res.body)
    if body == nil then
        body = res_body
    end
    if err ~= nil or res.status ~= 200 or res_body ~= body then
        local msg = "fail " ..
            tostring(err) ..
            " " ..
            tostring(res.status) ..
            " -" .. tostring(body) .. " -" .. tostring(res_body) .. "- " .. tostring(res_body == body)
        _M.fail(msg)
        return
    end
end

return _M
