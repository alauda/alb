---
--- Created by oilbeater.
--- DateTime: 17/9/13 下午3:28
--- A sample DSL to evaluate
---（AND (IN HOST www.baidu.com baidu.com) (EQ URL /search) (EQ SRC-IP 114.114.114.114))
---
local _M = {}

local function tokenizer(raw_dsl)
    local tokens = {}
    local next_token_beg = 1
    for i = 1, #raw_dsl do
        if(string.char(raw_dsl:byte(i)) == "(") then
            table.insert(tokens, "(")
            next_token_beg = i + 1
        elseif(string.char(raw_dsl:byte(i)) == " ") then
            local token = raw_dsl:sub(next_token_beg, i - 1)
            if(#token ~= 0 and token ~= " " and token ~= ")") then
                table.insert(tokens, token)
            end
            next_token_beg = i + 1
        elseif (string.char(raw_dsl:byte(i)) == ")") then
            local token = raw_dsl:sub(next_token_beg, i - 1)
            if(#token ~= 0 and token ~= " " and token ~= ")") then
                table.insert(tokens, token)
            end
            table.insert(tokens, ")")
            next_token_beg = i + 1
        end
    end
    return tokens
end

local function parse(tokens)
    if(#tokens == 0) then
        return nil, "unexpected EOF while parsing"
    end

    local token = table.remove(tokens, 1)
    if(token == "(") then
        local exp = {}
        while tokens[1] ~= ")" do
            table.insert(exp, parse(tokens))
        end
        table.remove(tokens, 1)
        return exp
    else
        return token
    end
end

local function eval(ast)
    local op = ast[1]
    local args = {}
    for i = 2, #ast do
        local token = ast[i]
        local token_type = type(token)
        if(token_type == "string") then
            table.insert(args, ast[i])
        elseif(token_type == "boolean") then
            if(op == "AND" and token == false) then
                return false, nil
            elseif(op == "OR" and token == true) then
                return true, nil
            else
                table.insert(args, ast[i])
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
                table.insert(args, result)
            else
                return false, err
            end
        end
    end
    local operation = require("operation")
    return operation.eval(op, args)
end

function _M.eval(rule)
    return eval(rule)
end

function _M.generate_ast(rule)
    return parse(tokenizer(rule))
end

return _M
