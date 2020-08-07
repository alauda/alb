local os_getenv = os.getenv
local io = io
local tonumber = tonumber
local string_format = string.format
local string_lower = string.lower
local common = require "common"
local dsl   = require "dsl"
local balancer = require "balancer"
local cache = require "cache"
local ngx = ngx
local ngx_shared = ngx.shared
local ngx_log = ngx.log
local ngx_exit = ngx.exit
local ngx_timer = ngx.timer
local ngx_worker = ngx.worker
local ngx_config = ngx.config

local subsystem = ngx_config.subsystem
local sync_policy_interval = tonumber(os_getenv("SYNC_POLICY_INTERVAL"))
-- /usr/local/openresty/nginx/conf/policy.new
local policy_path = os_getenv("NEW_POLICY_PATH")

local function fetch_policy()
    local f, err = io.open(policy_path, "r")
    if err then
        ngx_log(ngx.ERR, err)
        ngx_exit(ngx.ERROR)
    end
    local data = f:read("*a")
    if data == nil then
        ngx_log(ngx.ERR, "read policy file failed")
        return
    end
    f:close()
    local dict_data = common.json_decode(data)
    if dict_data == nil then
        ngx_log(ngx.ERR, "invalid policy file" .. data)
        return
    end
    local old_data = ngx_shared[subsystem .. "_raw"]:get("raw")
    local old_dict_data = common.json_decode(old_data)
    if common.table_equals(dict_data, old_dict_data) then
        return
    end
    ngx_shared[subsystem .. "_raw"]:set("raw", data)
    local changed_port_maps = {}
    local changed_cert_maps = {}
    if old_dict_data ~= nil then
        changed_port_maps = common.get_table_diff_keys(dict_data["port_map"], old_dict_data["port_map"])
        changed_cert_maps = common.get_table_diff_keys(dict_data["certificate_map"], old_dict_data["certificate_map"])
    end
    local all_ports_policies = dict_data["port_map"]
    local backend_group = dict_data["backend_group"]
    local certificate_map = dict_data["certificate_map"]
    ngx_shared[subsystem .. "_policy"]:set("all_policies", common.json_encode(all_ports_policies, true))
    ngx_shared[subsystem .. "_backend_cache"]:set("backend_group", common.json_encode(backend_group, true))

    if subsystem == "http" then
        ngx_shared[subsystem .. "_certs_cache"]:set("certificate_map", common.json_encode(certificate_map, true))
        for domain, certs in pairs(certificate_map) do
            local lower_domain = string_lower(domain)
            local clean_cache = false
            if common.table_contains(changed_cert_maps, domain) then
                clean_cache = true
                ngx_shared[subsystem .. "_certs_cache"]:delete(lower_domain)
            end
            ngx_shared[subsystem .. "_certs_cache"]:set(lower_domain, common.json_encode(certs))
            if clean_cache then
                cache.cert_cache:delete(lower_domain)
            end
        end
        -- NOTE: remove extra data
        for _, changed_domain in ipairs(changed_cert_maps) do
            local found = false
            for domain, _ in pairs(certificate_map) do
                if changed_domain == domain then
                    found = true
                end
            end
            if not found then
                local lower_domain = string_lower(changed_domain)
                ngx_shared[subsystem .. "_certs_cache"]:delete(lower_domain)
                cache.cert_cache:delete(lower_domain)
            end
        end
    end

    --split policies by port to decrease json operation overhead
    --parse raw dsl to ast to decrease overhead
    for port, policies in pairs(all_ports_policies) do
        local t = ""
        for _, policy in ipairs(policies) do
            if policy then
                t = policy["subsystem"]
                if t ~= subsystem then
                    break
                end
                if (policy["dsl"] and policy["dsl"] ~= "") or policy["internal_dsl"] ~= common.null then
                    if policy["internal_dsl"] ~= common.null then
                        if #policy["internal_dsl"] == 1 then
                            policy["dsl"]  = policy["internal_dsl"][1]
                        else
                            policy["dsl"]  = policy["internal_dsl"]
                        end
                    else
                        local tokenized_dsl, err = dsl.generate_ast(policy["dsl"])
                        if err then
                            ngx_log(ngx.ERR, "failed to generate ast for ", policy["dsl"], err)
                        else
                            policy["dsl"] = tokenized_dsl
                        end
                    end
                    --ngx.log(ngx.ERR, common.json_encode(policy["dsl"]))
                end
            end
        end
        do
            --[
            --  {
            --    "priority": 100,
            --    "rule": "rule_name_lorem",
            --    "upstream": "calico-new-yz-alb-09999-3a56db4e-20c3-42cb-82b8-fff848e8e6c3",
            --    "subsystem": "http",
            --    "url": "/s1",
            --    "dsl": [
            --      "AND",
            --      [
            --        "STARTS_WITH",
            --        "URL",
            --        "/s1"
            --      ]
            --    ],
            --    "rewrite_target": "/server_addr",
            --    "enable_cors": true,
            --    "backend_protocol": "https",
            --    "vhost": "baidu.com"
            --  }
            --]
        end
        if t == subsystem then
            local clean_cache = false
            if common.table_contains(changed_port_maps, port) then
                clean_cache = true
                ngx_shared[subsystem .. "_policy"]:delete(port)
            end
            ngx_shared[subsystem .. "_policy"]:set(port, common.json_encode(policies))
            if clean_cache then
                cache.rule_cache:delete(port)
            end
        end
    end
    -- NOTE: remove extra data
    for _, changed_port in ipairs(changed_port_maps) do
        local found = false
        for port, _ in pairs(all_ports_policies) do
            if changed_port == port then
                found = true
            end
        end
        if not found then
            ngx_shared[subsystem .. "_policy"]:delete(changed_port)
            cache.rule_cache:delete(changed_port)
        end
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
