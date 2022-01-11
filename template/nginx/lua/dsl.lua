---
--- Created by oilbeater.
--- DateTime: 17/9/13 下午3:28
--- A sample DSL to evaluate
---（AND (IN HOST www.baidu.com baidu.com) (EQ URL /search) (EQ SRC-IP 114.114.114.114))
local operation = require "operation"
local type = type
local string_char = string.char
local string_byte = string.byte
local string_sub = string.sub
local table_insert = table.insert
local table_remove = table.remove

local _M = {}

local function tokenizer(raw_dsl)
    local tokens = {}
    local next_token_beg = 1
    local insert_idx = 1
    for i = 1, #raw_dsl do
        if(string_char(string_byte(raw_dsl, i)) == "(") then
            tokens[insert_idx] = "("
            insert_idx = insert_idx + 1
            next_token_beg = i + 1
        elseif(string_char(string_byte(raw_dsl, i)) == " ") then
            local token = string_sub(raw_dsl, next_token_beg, i - 1)
            if(#token ~= 0 and token ~= " " and token ~= ")") then
                tokens[insert_idx] = token
                insert_idx = insert_idx + 1
            end
            next_token_beg = i + 1
        elseif (string_char(string_byte(raw_dsl, i)) == ")") then
            local token = string_sub(raw_dsl, next_token_beg, i - 1)
            if(#token ~= 0 and token ~= " " and token ~= ")") then
                tokens[insert_idx] = token
                insert_idx = insert_idx + 1
            end
            tokens[insert_idx] = ")"
            insert_idx = insert_idx + 1
            next_token_beg = i + 1
        end
    end
    return tokens
end

local function parse(tokens)
    if(#tokens == 0) then
        return nil, "unexpected EOF while parsing"
    end

    local token = table_remove(tokens, 1)
    if(token == "(") then
        local exp = {}
        while tokens[1] ~= ")" do
            local t, err = parse(tokens)
            if err then
                return nil, err
            end
            table_insert(exp, t)
        end
        table_remove(tokens, 1)
        return exp, nil
    else
        return token, nil
    end
end

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

function _M.generate_ast(rule)
    return parse(tokenizer(rule))
end

return _M
