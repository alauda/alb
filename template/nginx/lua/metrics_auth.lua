local _M = {}

local SKIP_METRICS_AURH = os.getenv("METRICS_AUTH") == "false"

local function get_token()
    local authorization = ngx.req.get_headers()['authorization']
    if type(authorization) ~= "string" then
        return nil
    end
    local matched = ngx.re.match(authorization, 'Bearer (.*)', 'jo')

    if matched == nil then
        return nil
    end
    local token = matched[1]
    if token == nil then
        return nil
    end
    return token
end

function _M.verify_auth()
    if SKIP_METRICS_AURH then
        return
    end
    local token = get_token()
    if token == nil then
        ngx.status = 401
        ngx.say("Unauthorized")
        ngx.exit(401)
    end
end

return _M
