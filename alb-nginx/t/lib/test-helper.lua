local _M = {}
local inspect = require"inspect"

function _M.trim(s) return s:gsub("^%s*(.-)%s*$", "%1") end

function _M.P(x) ngx.log(ngx.INFO, inspect.inspect(x)) end

-- test-helper should not use anything in common.lua,we just make a copy here.
function _M._table_equals(t1, t2, ignore_mt)
	local ty1 = type(t1)
	local ty2 = type(t2)
	if ty1 ~= ty2 then return false end
	-- non-table types can be directly compared
	if ty1 ~= "table" and ty2 ~= "table" then return t1 == t2 end
	-- as well as tables which have the metamethod __eq
	local mt = getmetatable(t1)
	if not ignore_mt and mt and mt.__eq then return t1 == t2 end
	for k1, v1 in pairs(t1) do
		local v2 = t2[k1]
		if v2 == nil or not _M._table_equals(v1, v2) then return false end
	end
	for k2, v2 in pairs(t2) do
		local v1 = t1[k2]
		if v1 == nil or not _M._table_equals(v1, v2) then return false end
	end
	return true
end

function _M.assert_table_equals(t1, t2, ignore_mt)
	if not _M._table_equals(t1, t2, ignore_mt) then
		ngx.log(ngx.ERR, "t1 != t2")
		_M.P(t1)
		_M.P(t2)
		ngx.exit(ngx.ERR)
	end
end

function _M.assert_contains(left, right)
	if left:find(right) then return true end
	ngx.log(ngx.ERR, "could not find " .. right .. " in " .. left)
	ngx.exit(ngx.ERR)
end

function _M.assert_eq(left, right, msg)
	local ty1 = type(left)
	local ty2 = type(right)
	if ty1 ~= ty2 then
		ngx.log(ngx.ERR, "type not same")
		ngx.exit(ngx.ERR)
	end
	if ty1 == ty2 and ty1 == "table" then
		_M.assert_table_equals(left, right)
		return
	end
	if not(left == right) then
		ngx.log(ngx.ERR, tostring(left) .. " ? " .. tostring(right) .. "  " .. tostring(left == right) .. " msg " .. tostring(msg) .. "\n")
		ngx.exit(ngx.ERR)
		return
	end
end

function _M.assert_true(v,msg)
	_M.assert_eq(v,true,msg)
end

--- a simple warpper of f which represents a testcase,you could enable or disable - a simple warpper of f which represents a testcase,you could enable or disable it.
-- TODO
function _M.testcase(title,enable,f)
	
end

function _M.assert_curl_success(res, err, body)
	if body == nil then body = "success" end
	if err ~= nil or res.status ~= 200 or res.body ~= body then
		ngx.log(ngx.ERR, "fail " .. tostring(err) .. " " .. tostring(res.status) .. " " .. tostring(res.body))
		ngx.exit(ngx.ERR)
		return
	end
end

return _M