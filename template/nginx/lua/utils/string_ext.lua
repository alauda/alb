local g_ext = require "utils.generic_ext"

local _M = {}

--- return true is str is nil or ""
function _M.is_nill(str)
    if str == nil then
        return true
    end
    if str == "" then
        return true
    end
    return false
end

---  nil_or return second if first is nil or empty string.
-- @return string
function _M.nil_or(first, second)
	return g_ext.nil_or(first, second,"")
end

return _M
