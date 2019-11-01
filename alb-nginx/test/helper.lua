---
--- Created by oilbeater.
--- DateTime: 17/11/23 下午2:52
---

local _M = {}

function _M.deepcopy(orig)
    local orig_type = type(orig)
    local copy
    if orig_type == 'table' then
        copy = {}
        for orig_key, orig_value in next, orig, nil do
            copy[_M.deepcopy(orig_key)] = _M.deepcopy(orig_value)
        end
        setmetatable(copy, _M.deepcopy(getmetatable(orig)))
    else -- number, string, boolean, etc
        copy = orig
    end
    return copy
end

return _M
