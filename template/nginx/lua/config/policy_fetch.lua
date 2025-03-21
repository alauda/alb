-- format:on
local os_getenv = os.getenv
local io = io
local string_lower = string.lower
local common = require "utils.common"
local cache = require "config.cache"
local subsys = require "utils.subsystem"
local shm = require "config.shmap"
local compress = require "utils.compress"

local ngx = ngx
local ngx_log = ngx.log

local HTTP_SUBSYSTEM = subsys.HTTP_SUBSYSTEM
local STREAM_SUBSYSTEM = subsys.STREAM_SUBSYSTEM

local _M = {}

local POLICY_PATH = os_getenv("NEW_POLICY_PATH")
local POLICY_ZIP = os_getenv("POLICY_ZIP")

local function set_default_value(table, key, default_val)
    if table[key] == nil then
        table[key] = default_val
    end
end

local function init_http_rule_dsl(rule)
    set_default_value(rule, "rewrite_base", "")
    set_default_value(rule, "rewrite_target", "")
    set_default_value(rule, "enable_cors", false)
    set_default_value(rule, "cors_allow_headers", "")
    set_default_value(rule, "cors_allow_origin", "")
    set_default_value(rule, "backend_protocol", "")
    set_default_value(rule, "redirect_url", "")
    set_default_value(rule, "vhost", "")
    -- we already have internal_dsl
    if rule["internal_dsl"] ~= common.null and rule["internal_dsl"] ~= nil then
        if #rule["internal_dsl"] == 1 then
            rule["dsl"] = rule["internal_dsl"][1]
        else
            rule["dsl"] = rule["internal_dsl"]
        end
    else
        ngx_log(ngx.ERR, "internal_dsl is null " .. tostring(rule["rule"]))
    end
end

--- convert a file path to it string content.
--- if error, return [nil,string(an error message)].
--- if success, return [string(file content),nil]
---@param path string
---@return string|nil,string|nil
local function file_read_to_string(path)
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

local function update_stream_policy_cache(policy, old_policy, protocol)
    for port, reason in pairs(common.get_table_diff_keys(policy, old_policy)) do
        local key = cache.gen_rule_key(STREAM_SUBSYSTEM, protocol, port)
        if reason == common.DIFF_KIND_REMOVE then
            shm.del_stream_policy(key)
            cache.rule_cache:delete(key)
        end
        if reason == common.DIFF_KIND_ADD or reason == common.DIFF_KIND_CHANGE then
            ngx.log(ngx.INFO, string.format("set ngx.share[stream_policy][%s]", key))
            local policy_json_str = common.json_encode(policy[port], true)
            shm.set_stream_policy(key, policy_json_str)
        end
    end
end

--- this function only be called in stream subsystem
---@param policy table
---@param old_policy table
local function update_stream_cache(policy, old_policy)
    local backend_group = common.access_or(policy, { "backend_group" }, {})
    shm.set_backends(common.json_encode(backend_group, true))

    local stream_tcp_policy = common.access_or(policy, { "stream", "tcp" }, {})
    local old_stream_tcp_policy = common.access_or(old_policy, { "stream", "tcp" }, {})
    update_stream_policy_cache(stream_tcp_policy, old_stream_tcp_policy, "tcp")

    local stream_udp_policy = common.access_or(policy, { "stream", "udp" }, {})
    local old_stream_udp_policy = common.access_or(old_policy, { "stream", "udp" }, {})
    update_stream_policy_cache(stream_udp_policy, old_stream_udp_policy, "udp")
end

---@param policy table|nil
---@param old_policy table|nil
local function update_http_cache(policy, old_policy)
    local certificate_map = common.access_or(policy, { "certificate_map" }, {})
    local old_certificate_map = common.access_or(old_policy, { "certificate_map" }, {})

    local http_policy = common.access_or(policy, { "http", "tcp" }, {})
    local old_http_policy = common.access_or(old_policy, { "http", "tcp" }, {})

    local backend_group = common.access_or(policy, { "backend_group" }, {})
    shm.set_backends(common.json_encode(backend_group, true))

    local new_config = common.access_or(policy, { "config" }, {})
    local old_config = common.access_or(old_policy, { "config" }, {})
    -- ngx.log(ngx.ERR, string.format("newconfig %s", common.json_encode(new_config, true)))
    -- ngx.log(ngx.ERR, string.format("oldconfig %s", common.json_encode(old_config, true)))
    -- ngx.log(ngx.ERR, string.format("new policy %s", common.json_encode(http_policy, true)))
    -- ngx.log(ngx.ERR, string.format("old policy %s", common.json_encode(old_http_policy, true)))

    -- NOTICE: The value argument inserted can be Lua booleans, numbers, strings, or nil.
    -- since that we have to insert a json string....

    -- update cert cache
    -- TODO get_table_diff_keys 是递归的。。可能特别慢
    for domain, reason in pairs(common.get_table_diff_keys(certificate_map, old_certificate_map)) do
        local lower_domain = string_lower(domain)
        if reason == common.DIFF_KIND_REMOVE then
            ngx.log(ngx.INFO, "cert delete domain " .. lower_domain)
            shm.del_http_cert(lower_domain)
            cache.cert_cache:delete(lower_domain)
        end
        if reason == common.DIFF_KIND_ADD or reason == common.DIFF_KIND_CHANGE then
            ngx.log(ngx.INFO, "cert set domain " .. lower_domain)
            cache.cert_cache:delete(lower_domain)
            shm.set_http_cert(lower_domain, common.json_encode(certificate_map[domain]))
        end
    end

    -- update policy cache
    for port, reason in pairs(common.get_table_diff_keys(http_policy, old_http_policy)) do
        -- we only support http via tcp now.
        local key = cache.gen_rule_key(HTTP_SUBSYSTEM, "tcp", port)
        if reason == common.DIFF_KIND_REMOVE then
            ngx.log(ngx.INFO, "cache: http policy delete key " .. key)
            shm.del_http_rule(key)
            cache.rule_cache:delete(key)
        end

        if reason == common.DIFF_KIND_ADD or reason == common.DIFF_KIND_CHANGE then
            for _, rule in ipairs(http_policy[port]) do
                init_http_rule_dsl(rule)
            end
            ngx.log(ngx.INFO, string.format("set ngx.share[http_policy][%s]", key))
            shm.set_http_rule(key, common.json_encode(http_policy[port]))
            cache.rule_cache:delete(key)
        end
    end
    -- TODO config的key是hash，我们不用比较完整的配置
    -- update config cache
    for name, reason in pairs(common.get_table_diff_keys(new_config, old_config)) do
        --- 更新cache，这样下次就会直接从shdict中读新的值
        cache.remove_config(name)
        if reason == common.DIFF_KIND_REMOVE then
            shm.del_config(name)
        end
        if reason == common.DIFF_KIND_ADD or reason == common.DIFF_KIND_CHANGE then
            ngx.log(ngx.INFO, string.format("set ngx.share[http_policy][%s]", name))
            shm.set_config(name, common.json_encode(new_config[name]))
        end
    end
end

function _M.update_policy(policy_raw, via)
    if via == nil then
        via = "auto"
    end
    if policy_raw == "" then
        return "empty policy " .. via
    end
    if policy_raw == nil then
        return "empty policy " .. via
    end

    local policy_data = common.json_decode(policy_raw)
    if policy_data == nil then
        return "invalid policy data " .. via .. " " .. policy_raw
    end

    local old_policy_raw = shm.get_policy_raw()
    local old_policy_data = common.json_decode(old_policy_raw)
    if old_policy_raw == policy_raw then
        return
    end
    if common.table_equals(policy_data, old_policy_data) then
        return
    end
    shm.set_policy_raw(policy_raw)

    if subsys.is_http_subsystem() then
        update_http_cache(policy_data, old_policy_data)
        return
    end

    if subsys.is_stream_subsystem() then
        update_stream_cache(policy_data, old_policy_data)
        return
    end
end

function _M.fetch_policy()
    local policy_raw
    local err
    if POLICY_ZIP == "true" then
        policy_raw, err = compress.decompress_from_file(POLICY_PATH .. ".bin")
    else
        policy_raw, err = file_read_to_string(POLICY_PATH)
    end
    if err ~= nil then
        ngx_log(ngx.ERR, "read policy in " .. POLICY_PATH .. " zip " .. tostring(POLICY_ZIP) .. " fail " .. err)
        return
    end
    local err = _M.update_policy(policy_raw, "auto")
    if err ~= nil then
        ngx_log(ngx.ERR, "fetch policy fail, policy path" .. POLICY_PATH .. " err " .. err)
    end
end

return _M
