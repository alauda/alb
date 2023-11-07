local _M = {}

local inspect = require "inspect"

function _M.httpc()
    return require "resty.http".new()
end

function _M.curl(url, cfg)
    local httpc = require("resty.http").new()
    if cfg == nil then
        cfg = {}
        cfg["headers"] = {}
    end
    local res, err = httpc:request_uri(url, { method = "GET", headers = cfg.headers })
    return res, err
end

_M.inspect = inspect
---comment
---@param msg any # will be warpped with tostring
---@param opt? table
function _M.log(msg, opt)
    local caller = ""
    if opt ~= nil then
        caller = tostring(opt.caller)
    end
    ngx.log(ngx.NOTICE, "\n---alb_debug " .. caller .. "---\n " .. tostring(msg) .. "\n---alb_debug_end---\n")
end

---comment
---@param arg []any  # will be combine and wrapped with inspect
function _M.logs(...)
    local callerinfo = debug.getinfo(2)
    local caller = callerinfo.source .. " " .. tostring(callerinfo.currentline)
    local msg = ""
    local t, n = { ... }, select('#', ...)
    for k = 1, n do
        local v = t[k]
        msg = msg .. " |> " .. inspect(v) .. " <|"
    end
    -- local list = table.pack(...)
    -- local msg = ""
    -- for i, v in ipairs(list) do
    --     msg = msg .. " |->" .. inspect(v) .. "<-|"
    -- end
    -- _M.log(inspect(arg), { caller = caller })
    _M.log(msg, { caller = caller })
end

function _M.now_ms()
    ngx.update_time()
    return ngx.now() * 1000
end

function _M.time_spend(f)
    local start = _M.now_ms()
    _M.log("time spend start " .. tostring(ngx.now()))
    local ret = { f() }
    local stop = _M.now_ms()
    _M.log("time spend end " .. tostring(ngx.now()))
    return stop - start, unpack(ret)
end

function _M.shell(cmd)
    local shell = require "resty.shell"
    local ok, stdout, stderr = shell.run(cmd)
    return stdout, stderr
end

function _M.shell_curl(cmd)
    local shell = require "resty.shell"
    local ok, stdout, stderr = shell.run(cmd)
    return stdout, stderr
end

function _M.file_read_to_string(path)
    local f, err = io.open(path, "r")
    if err then
        return nil, err
    end
    local raw = f:read("*a")
    f:close()
    if raw == nil then
        return nil, "could not read file content"
    end
    return raw, nil
end

return _M
