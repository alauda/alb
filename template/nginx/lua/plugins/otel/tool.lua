local attr                     = require("opentelemetry.attribute")

local _M = {}
---insert string attribute to table if val is not nil
---@param m table
---@param key string
---@param val? string
function _M.attribute_set_string(m, key, val)
    if val == nil then
        return
    end
    table.insert(m, attr.string(key, val))
end

return _M
