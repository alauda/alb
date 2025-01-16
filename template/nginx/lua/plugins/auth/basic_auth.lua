local _m = {}
local ngx = ngx
local crypt = require "plugins.auth.crypt"
local decode_base64 = ngx.decode_base64
local alb_err = require "error"
local s_ext = require("utils.string_ext")
local lrucache = require "resty.lrucache"

local lru, err = lrucache.new(500)
if not lru then
    error("failed to create the cache: " .. (err or "unknown"))
end

---comment
---@param auth_header string
---@return string user
---@return string pass
---@return string? err
local function parse_basic_auth(auth_header)
    if not s_ext.start_with(auth_header, "Basic") then
        return "", "", "invalid Authorization scheme, not basic auth."
    end

    local decoded = decode_base64(s_ext.remove_prefix(auth_header, "Basic "))
    if not decoded then
        return "", "", "invalid base64 encoding"
    end
    local user_pass = s_ext.split(decoded, ":")
    if #user_pass ~= 2 then
        return "", "", "invalid format"
    end
    local user, pass = user_pass[1], user_pass[2]
    return user, pass, nil
end


---comment
---@param auth_cfg AuthPolicy
---@param ctx AlbCtx
_m.do_basic_auth_if_need = function (auth_cfg, ctx)
    if auth_cfg.basic_auth == nil then
        return
    end
    if auth_cfg.basic_auth.err ~= "" then
        alb_err.exit_with_code(alb_err.AUTHFAIL, "invalid cfg " .. auth_cfg.basic_auth.err, 500)
    end

    ngx.header["WWW-Authenticate"] = "Basic realm=" .. "\"" .. auth_cfg.basic_auth.realm .. "\""

    local secrets = auth_cfg.basic_auth.secret
    local auth_header = ctx.var["http_authorization"]
    if auth_header == nil then
        alb_err.exit_with_code(alb_err.AUTHFAIL, "basic_auth but req no auth header", 401)
    end
    local user, pass, err = parse_basic_auth(auth_header)
    if err ~= nil then
        alb_err.exit_with_code(alb_err.AUTHFAIL, err, 401)
    end
    -- find hash for this user
    if secrets[user] == nil then
        alb_err.exit_with_code(alb_err.AUTHFAIL, "invalid user or passwd", 401)
    end
    local ok = _m.verify(pass, secrets[user])
    if ok then
        ngx.header["WWW-Authenticate"] = nil
    else
        alb_err.exit_with_code(alb_err.AUTHFAIL, "invalid user or passwd", 401)
    end
end



-- apr1是性能损耗比较大的一个操作
-- 我们的期望是在某次成功之后后续就不应该在重复计算了
---@param pass string
---@param hash BasicAuthHash
_m.verify = function (pass, hash)
    local cache_key = hash.hash
    local cache_pass = lru:get(cache_key)
    if cache_pass ~= nil and cache_pass ~= pass then
        return false
    end
    local calculate_hash = crypt.apr1(pass, hash.salt)

    if calculate_hash == hash.hash then
        lru:set(cache_key, pass, 60 * 60)
        return true
    end
    return false
end
return _m
