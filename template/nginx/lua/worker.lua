local os_getenv = os.getenv
local io = io
local tonumber = tonumber
local string_format = string.format
local string_lower = string.lower
local common = require "utils.common"
local balancer = require "balancer"
local cache = require "cache"
local ngx = ngx
local ngx_shared = ngx.shared
local ngx_log = ngx.log
local ngx_timer = ngx.timer
local ngx_worker = ngx.worker
local ngx_config = ngx.config

local current_subsystem = ngx_config.subsystem

local HTTP_SUBSYSTEM = "http"
local STREAM_SUBSYSTEM = "stream"

if current_subsystem == "http" then
    require "init_l7"
else
    require "init_l4"
end

local sync_policy_interval = tonumber(os_getenv("SYNC_POLICY_INTERVAL"))

-- /usr/local/openresty/nginx/conf/policy.new
local POLICY_PATH = os_getenv("NEW_POLICY_PATH")

local clean_metrics_interval = tonumber(os_getenv("CLEAN_METRICS_INTERVAL"))

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
    if rule["internal_dsl"] ~= common.null then
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
            ngx_shared["stream_policy"]:delete(key)
            cache.rule_cache:delete(key)
        end
        if reason == common.DIFF_KIND_ADD or reason == common.DIFF_KIND_CHANGE then
            ngx.log(ngx.INFO, string.format("set ngx.share[stream_policy][%s]", key))
            local policy_json_str = common.json_encode(policy[port], true)
            ngx_shared["stream_policy"]:set(key, policy_json_str)
        end
    end
end

--- this function only be called in stream subsystem
---@param policy table
---@param old_policy table
local function update_stream_cache(policy, old_policy)
    local backend_group = common.access_or(policy, {"backend_group"}, {})
    local stream_policy = common.access_or(policy, {"stream"}, {})

    ngx_shared["stream_backend_cache"]:set("backend_group", common.json_encode(backend_group, true))
    ngx_shared["stream_policy"]:set("all_policies", common.json_encode(stream_policy, true))

    local stream_tcp_policy = common.access_or(policy, {"stream", "tcp"}, {})
    local old_stream_tcp_policy = common.access_or(old_policy, {"stream", "tcp"}, {})
    update_stream_policy_cache(stream_tcp_policy, old_stream_tcp_policy, "tcp")

    local stream_udp_policy = common.access_or(policy, {"stream", "udp"}, {})
    local old_stream_udp_policy = common.access_or(old_policy, {"stream", "udp"}, {})
    update_stream_policy_cache(stream_udp_policy, old_stream_udp_policy, "udp")
end

---@param policy table|nil
---@param old_policy table|nil
local function update_http_cache(policy, old_policy)
    local backend_group = common.access_or(policy, {"backend_group"}, {})
    local certificate_map = common.access_or(policy, {"certificate_map"}, {})
    local old_certificate_map = common.access_or(old_policy, {"certificate_map"}, {})

    local http_policy = common.access_or(policy, {"http", "tcp"}, {})
    local old_http_policy = common.access_or(old_policy, {"http", "tcp"}, {})

    ngx_shared["http_backend_cache"]:set("backend_group", common.json_encode(backend_group, true))
    ngx_shared["http_certs_cache"]:set("certificate_map", common.json_encode(certificate_map, true))
    ngx_shared["http_policy"]:set("all_policies", common.json_encode(http_policy, true))

    -- update cert cache
    for domain, reason in pairs(common.get_table_diff_keys(certificate_map, old_certificate_map)) do
        local lower_domain = string_lower(domain)
        if reason == common.DIFF_KIND_REMOVE then
            ngx.log(ngx.INFO, "cert delete domain " .. lower_domain)
            ngx_shared["http_certs_cache"]:delete(lower_domain)
            cache.cert_cache:delete(lower_domain)
        end
        if reason == common.DIFF_KIND_ADD or reason == common.DIFF_KIND_CHANGE then
            ngx.log(ngx.INFO, "cert set domain " .. lower_domain)
            cache.cert_cache:delete(lower_domain)
            ngx_shared["http_certs_cache"]:set(lower_domain, common.json_encode(certificate_map[domain]))
        end
    end

    -- update policy cache
    for port, reason in pairs(common.get_table_diff_keys(http_policy, old_http_policy)) do
        -- we only support http via tcp now.
        local key = cache.gen_rule_key(HTTP_SUBSYSTEM, "tcp", port)
        if reason == common.DIFF_KIND_REMOVE then
            ngx.log(ngx.INFO, "cache: http policy delete key " .. key)
            ngx_shared["http_policy"]:delete(key)
            cache.rule_cache:delete(key)
        end

        if reason == common.DIFF_KIND_ADD or reason == common.DIFF_KIND_CHANGE then
            for _, rule in ipairs(http_policy[port]) do
                init_http_rule_dsl(rule)
            end
            ngx.log(ngx.INFO, string.format("set ngx.share[http_policy][%s]", key))
            cache.rule_cache:delete(key)
            ngx_shared["http_policy"]:set(key, common.json_encode(http_policy[port]))
        end
    end
end

---@return boolean
local function is_http_subsystem()
    return current_subsystem == HTTP_SUBSYSTEM
end

---@return boolean
local function is_stream_subsystem()
    return current_subsystem == STREAM_SUBSYSTEM
end

local function _fetch_policy(policy_raw)
    if policy_raw == "" then
        return "empty policy"
    end

    local policy_data = common.json_decode(policy_raw)
    if policy_data == nil then
        return "invalid policy data " .. policy_raw
    end

    local old_policy_raw = ngx_shared[current_subsystem .. "_raw"]:get("raw")
    local old_policy_data = common.json_decode(old_policy_raw)

    if common.table_equals(policy_data, old_policy_data) then
        return
    end
    ngx_shared[current_subsystem .. "_raw"]:set("raw", policy_raw)

    if is_http_subsystem() then
        update_http_cache(policy_data, old_policy_data)
        return
    end

    if is_stream_subsystem() then
        update_stream_cache(policy_data, old_policy_data)
        return
    end
end

local function fetch_policy()
    local policy_raw, err = file_read_to_string(POLICY_PATH)
    if err ~= nil then
        ngx_log(ngx.ERR, "read policy in " .. POLICY_PATH .. " fail " .. err)
    end
    local err = _fetch_policy(policy_raw)
    if err ~= nil then
        ngx_log(ngx.ERR, "fetch policy fail, policy path" .. POLICY_PATH .. " err " .. err)
    end
end

if ngx_worker.id() == 0 then
    -- master sync policy
    fetch_policy()
    local _, err = ngx_timer.every(sync_policy_interval, fetch_policy)
    if err then
        ngx_log(ngx.ERR, string_format("error when setting up timer.every for fetch_policy: %s", tostring(err)))
    end
end

-- worker keep upstream peer balanced
balancer.sync_backends()
local _, err = ngx_timer.every(sync_policy_interval, balancer.sync_backends)
if err then
    ngx_log(ngx.ERR, string_format("error when setting up timer.every for sync_backends: %s", tostring(err)))
end

local clean_metrics
clean_metrics = function(premature)
    if premature then
        return
    end
    if is_http_subsystem() then
        require("metrics").clear()
    end
end

-- worker clean up metrics data for prometheus periodically
if ngx_worker.id() == 0 then
    -- master clean up metrics data
    local _, err = ngx_timer.every(clean_metrics_interval, clean_metrics)
    if err then
        ngx_log(ngx.ERR, string_format("error when setting up timer.every for clean_metrics: %s", tostring(err)))
    end
end
