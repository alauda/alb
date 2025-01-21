local _m = {}

local clone = require "table.clone"
local u_url = require "utils.url"
local alb_err = require "error"

function _m.do_forward_auth_if_need(auth_cfg, ctx)
    if not auth_cfg.forward_auth then
        return
    end

    local ret = _m.send_auth_request(auth_cfg.forward_auth, ctx)
    ngx.log(ngx.INFO, "auth-success: ", tostring(ret.success),
        " url: " .. tostring(ret.url) .. " code: " .. tostring(ret.code) .. " err: " .. tostring(ret.error_reason))
    -- auth fail
    if not ret.success then
        for k, v in pairs(ret.client_response_extra_headers) do
            ngx.header[k] = v
        end
        if auth_cfg.forward_auth.always_set_cookie then
            ngx.header["Set-Cookie"] = ret.cookie
        end
        if ret.code ~= 302 then
            alb_err.exit_with_code(alb_err.AUTHFAIL, ret.error_reason, ret.code)
            ngx.say(ret.body)
            return
        end
        ngx.exit(302)
        ngx.say(ret.body)
        return
    end

    -- auth success
    -- always_set_cookie need auth ctx in response_header_filter
    ctx.auth = {
        always_set_cookie = auth_cfg.forward_auth.always_set_cookie,
        auth_cookie = ret.cookie
    }
    for k, v in pairs(ret.upstream_req_extra_headers) do
        ngx.req.set_header(k, v)
    end
    for k, v in pairs(ret.client_response_extra_headers) do
        ngx.header[k] = v
    end
end

local valid_code = {
    ["200"] = true,
    ["201"] = true,
    ["204"] = true,
    ["206"] = true,
    ["301"] = true,
    ["302"] = true,
    ["303"] = true,
    ["304"] = true,
    ["307"] = true,
    ["308"] = true
}

function _m.merge_cookie(ca, cb)
    if ca == nil and cb == nil then
        return
    end
    if ca == nil then
        return cb
    end
    if cb == nil then
        return ca
    end
    if type(ca) == "string" then
        ca = { ca }
    end
    if type(cb) == "string" then
        cb = { cb }
    end
    if type(ca) ~= "table" or type(cb) ~= "table" then
        ngx.log(ngx.ERR, "invalid type of ca or cb: ", type(ca), type(cb))
        return
    end

    local cookies = {}
    for _, c in ipairs(ca) do
        table.insert(cookies, c)
    end
    for _, c in ipairs(cb) do
        table.insert(cookies, c)
    end
    return cookies
end

---@param ctx AlbCtx
function _m.add_cookie_if_need(ctx)
    local should_add_cookie = ctx.auth.auth_cookie ~= nil and
        (valid_code[tostring(ngx.var.status)] or ctx.auth.always_set_cookie)
    if not should_add_cookie then
        return
    end
    ngx.header["Set-Cookie"] = _m.merge_cookie(ngx.header["Set-Cookie"], ctx.auth.auth_cookie)
end

--- type
---@class AuthAction
---@field success boolean
---@field error_reason string
---@field url string
---@field upstream_req_extra_headers table<string,string> # bypass的情况下，在发送给upstream的请求中需要增加的header
---@field client_response_extra_headers table<string,string> # 给client的response中需要增加的header
---@field cookie? table # auth response中的cookie
---@field code integer                 # !success的情况下，返回给客户端的code
---@field body any?                    # !success的情况下，返回给客户端的body
-- type

---@param auth ForwardAuthPolicy
---@param ctx AlbCtx
---@return AuthAction
function _m.send_auth_request(auth, ctx)
    ---@type AuthAction
    local ret = {
        success = false,
        error_reason = "",
        url = "",
        upstream_req_extra_headers = {},
        client_response_extra_headers = {},
        code = 500,
        body = "ALB Auth Fail",
    }

    if auth.invalid_auth_req_cm_ref then
        ret.code = 503
        ret.error_reason = "invalid-auth-req-cm-ref"
        return ret
    end

    local req, err = _m.build_request(auth, ctx.var)
    if err ~= nil then
        ngx.log(ngx.ERR, "do auth, build request fail: ", err)
        ret.error_reason = "build-auth-request-fail"
        return ret
    end
    ret.url = req.url

    local httpc = require("resty.http").new()
    httpc:set_timeout(5 * 1000) -- TODO timeout
    local res, err = httpc:request_uri(req.url, req)
    if err ~= nil then
        ngx.log(ngx.ERR, "do auth, send request fail: ", tostring(err))
        ret.error_reason = "send-auth-request-fail"
        return ret
    end
    ret.cookie = res.headers["Set-Cookie"]
    -- success
    if res.status >= 200 and res.status < 300 then
        ret.success = true
        ret.code = res.status
        for _, h in ipairs(auth.upstream_headers) do
            if res.headers[h] ~= nil then
                ret.upstream_req_extra_headers[h] = res.headers[h]
            end
        end
        return ret
    end

    -- auth fail 401
    if res.status == 401 then
        -- TODO 这里的大小写没问题吗...
        ret.client_response_extra_headers["WWW-Authenticate"] = res.headers["WWW-Authenticate"]
        ret.body = res.body
        ret.code = res.status
        -- with redirect
        if #auth.signin_url ~= 0 then
            local url, err = _m.resolve_varstring(auth.signin_url, ctx.var)
            if err ~= nil then
                ret.error_reason = "resolve-signinurl-fail"
                return ret
            end
            ret.client_response_extra_headers["Location"] = url
            ret.code = 302
            return ret
        end
        ret.error_reason = "auth-service-status: " .. tostring(res.status)
        return ret
    end
    ret.error_reason = "auth-service-status: " .. tostring(res.status)
    if res.status == 403 then
        ret.code = res.status
        ret.body = res.body
        return ret
    end
    -- other fail
    ret.code = 500
    return ret
end

---comment
---@param auth ForwardAuthPolicy
---@return table
---@return string? error
function _m.build_request(auth, var)
    local url, err = _m.resolve_varstring(auth.url, var)
    if err ~= nil then
        return {}, err
    end
    local parts, err = u_url.parse(url)
    if err ~= nil then
        return {}, tostring(err)
    end
    if parts == nil then
        return {}, "invalid url: " .. url
    end
    local auth_host = parts.host

    local req = {
        method = auth.method,
        url = url,
        headers = {},
        ssl_verify = false,
    }

    local default_headers = {
        ["X-Original-URI"] = { "$request_uri" },
        ["X-Scheme"] = { "$pass_access_scheme" },
        ["X-Original-URL"] = { "$scheme", "://", "$http_host", "$request_uri" },
        ["X-Original-Method"] = { "$request_method" },
        ["X-Sent-From"] = { "alb" },
        ["Host"] = { auth_host },
        ["X-Real-IP"] = { "$remote_addr" },
        ["X-Forwarded-For"] = { "$proxy_add_x_forwarded_for" },
        ["X-Auth-Request-Redirect"] = { "$request_uri" },
        ["Connection"] = { "close" }, -- explicit close. we do not support auth keep-alive now.
    }
    for k, v in pairs(ngx.req.get_headers()) do
        req.headers[k] = v
    end
    for k, v in pairs(default_headers) do
        req.headers[k] = _m.resolve_varstring(v, var)
    end
    for k, v in pairs(auth.auth_headers) do
        local rv, err = _m.resolve_varstring(v, var)
        if err ~= nil then
            return {}, err
        end
        req.headers[k] = rv
    end
    if auth.auth_request_redirect then
        local redirect_url, err = _m.resolve_varstring(auth.auth_request_redirect, var)
        if err ~= nil then
            return {}, err
        end
        if redirect_url ~= "" then
            req.headers["X-Auth-Request-Redirect"] = redirect_url
        end
    end
    return req
end

---comment turn $host_xx into real value
---@param str_template_list string[]
---@param var table<string,string>
---@return string
---@return string? error
function _m.resolve_varstring(str_template_list, var)
    -- ["https://","$host","/oauth2/auth"]
    -- ["https://","$host","/oauth2/start?rd=","$escaped_request_uri"]
    -- don't modify the original table
    local str_list = clone(str_template_list)
    for index, v in pairs(str_list) do
        if string.sub(v, 1, 1) == "$" and v ~= "$" then
            local key = string.sub(v, 2)
            local resolved_val = var[key]
            -- 如果有其他变量的话，后面可能需要扩展下.. 比如把escape 作为通用的前缀
            if key == "escaped_request_uri" then
                resolved_val = ngx.escape_uri(var["request_uri"])
            end
            if key == "pass_access_scheme" then
                resolved_val = var["scheme"]
            end
            if resolved_val == nil then
                return "", "var " .. key .. " not found"
            end
            str_list[index] = resolved_val
        end
    end
    return table.concat(str_list), nil
end

return _m
