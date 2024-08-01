---
--- Created by oilbeater.
--- DateTime: 17/9/13 下午3:28
--- A sample DSL to evaluate
---（AND (IN HOST www.baidu.com baidu.com) (EQ URL /search) (EQ SRC-IP 114.114.114.114))
local operation = require "match_engine.operation"
local type = type

local _M = {}

local function eval(ast)
    local op = ast[1]
    local args = {}
    local insert_idx = 1
    for i = 2, #ast do
        local token = ast[i]
        local token_type = type(token)
        if(token_type == "string") then
            args[insert_idx]  = ast[i]
        elseif(token_type == "boolean") then
            if(op == "AND" and token == false) then
                return false, nil
            elseif(op == "OR" and token == true) then
                return true, nil
            else
                args[insert_idx]  = ast[i]
            end
        elseif(token_type == "table") then
            local result, err = eval(ast[i])
            if(err == nil) then
                if(type(result) == "boolean") then
                    if(op == "AND" and result == false) then
                        return false, nil
                    elseif(op == "OR" and result == true) then
                        return true, nil
                    end
                end
                args[insert_idx]  = result
            else
                return false, err
            end
        end
        insert_idx = insert_idx + 1
    end
    return operation.eval(op, args)
end

function _M.eval(rule)
    return eval(rule)
end

return _M
