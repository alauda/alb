local _m = {}
local s_ext = require("utils.string_ext")

--- @param x_forwarded_for string
--- @return string|nil
function _m.parse_first_forward(x_forwarded_for)
    if type(x_forwarded_for) ~= "string" then
        return nil
    end
    local ips = s_ext.split(x_forwarded_for, ",")
    if #ips > 0 and ips[1] ~= "" then
        return ips[1]
    end
    return nil
end

return _m
