local _M = {}


---  nil_or return second if first is nil or empty string.
-- @return string
function _M.nil_or(first, second,empty)
    if first == nil then
        return second
    end
    if first == empty then
        return second
    end
    return first
end

return _M