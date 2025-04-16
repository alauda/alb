local _m = {}
local s_ext = require("utils.string_ext")

--- @param x_forwarded_for string
--- @return string|nil
function _m.parse_first_forward(x_forwarded_for)
    if type(x_forwarded_for) ~= "string" then
        return nil
    end

    -- 查找第一个逗号的位置
    local comma_pos = string.find(x_forwarded_for, ",", 1, true)

    local first_part
    if comma_pos then
        -- 如果找到逗号，提取逗号之前的部分
        first_part = string.sub(x_forwarded_for, 1, comma_pos - 1)
    else
        first_part = x_forwarded_for
    end

    -- 去除提取部分的头尾空白
    local trimmed_ip = s_ext.trim_space(first_part)
    if trimmed_ip ~= "" then
        return trimmed_ip
    end
    return nil
end

return _m
