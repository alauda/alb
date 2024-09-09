-- format:on
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
    return g_ext.nil_or(first, second, "")
end
-- remove_prefix
--  if prefix not exist return the origin str
-- @return string
function _M.remove_prefix(s, prefix)
    local sub = string.sub
    local len = #s
    local plen = #prefix
    if len == 0 or plen == 0 or len < plen then
        return s
    elseif s == prefix then
        return ""
    elseif sub(s, 1, plen) == prefix then
        return sub(s, plen + 1)
    end
    return s
end

--- split string to lines and grep the lines with regex
--- @param s string
--- @param regex string
--- @return string[]
function _M.lines_grep(s, regex)
    local lines = {}
    for line in string.gmatch(s, "[^\r\n]+") do
        if string.match(line, regex) then
            table.insert(lines, line)
        end
    end
    return lines
end

return _M
