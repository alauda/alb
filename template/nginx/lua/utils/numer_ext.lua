local g_ext = require "utils.generic_ext"

local _M = {}

---  nil_or return second if first is nil or empty string.
-- @return string
function _M.nil_or(first, second)
    return g_ext.nil_or(first, second, 0)
end

return _M
