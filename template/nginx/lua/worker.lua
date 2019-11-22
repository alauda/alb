local os_getenv = os.getenv
local io = io
local tonumber = tonumber
local string_format = string.format
local string_lower = string.lower
local common = require "common"
local dsl   = require "dsl"
local balancer = require "balancer"
local ngx = ngx
local ngx_shared = ngx.shared
local ngx_log = ngx.log
local ngx_exit = ngx.exit
local ngx_timer = ngx.timer
local ngx_worker = ngx.worker
local ngx_config = ngx.config

local subsystem = ngx_config.subsystem
local sync_policy_interval = tonumber(os_getenv("SYNC_POLICY_INTERVAL"))
local sync_backend_interval = tonumber(os_getenv("SYNC_BACKEND_INTERVAL"))
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
    local old_data = ngx_shared[subsystem .. "_alb_cache"]:get("raw")
    local dict_data = common.json_decode(data)
    local old_dict_data = common.json_decode(old_data)
    if common.table_equals(dict_data, old_dict_data) then
        return
    end
    ngx_log(ngx.ERR, "policy changed, update")
    ngx_shared[subsystem .. "_alb_cache"]:set("raw", data)
    local all_ports_policies = dict_data["port_map"]
    local backend_group = dict_data["backend_group"]
    ngx_shared[subsystem .. "_policy"]:set("all_policies", common.json_encode(all_ports_policies, true))
    ngx_shared[subsystem .. "_backend_cache"]:set("backend_group", common.json_encode(backend_group, true))

    if subsystem == "http" then
        local certificate_map = dict_data["certificate_map"]
        ngx_shared[subsystem .. "_certs_cache"]:set("certificate_map", common.json_encode(certificate_map, true))
        for domain, certs in pairs(certificate_map) do
            ngx_shared[subsystem .. "_certs_cache"]:set(string_lower(domain), common.json_encode(certs))
        end
    end

    --split policies by port to decrease json operation overhead
    --parse raw dsl to ast to decrease overhead
    for port, policies in pairs(all_ports_policies) do
        local t = ""
        for _, policy in ipairs(policies) do
            if policy then
                t = policy["subsystem"]
                if policy["dsl"] and policy["dsl"] ~= "" then
                    --ngx.log(ngx.ERR, common.json_encode(policy["dsl"]))
                    local new_rule, err = dsl.generate_ast(policy["dsl"])
                    if err then
                        ngx_log(ngx.ERR, "failed to generate ast for ", policy["dsl"], err)
                    else
                        policy["dsl"] = new_rule
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
            --    "rewrite_target": "/server_addr"
            --  }
            --]
        end
        if t == subsystem then
            ngx_shared[subsystem .. "_policy"]:set(port, common.json_encode(policies))
        end
    end
    for _, backend in ipairs(backend_group) do
        do
            --{
            --  "mode": "http",
            --  "session_affinity_attribute": "",
            --  "name": "calico-new-yz-alb-09999-3a56db4e-20c3-42cb-82b8-fff848e8e6c3",
            --  "session_affinity_policy": "",
            --  "backends": [
            --    {
            --      "port": 80,
            --      "address": "10.16.12.9",
            --      "weight": 100
            --    }
            --  ]
            --}
        end
        ngx_shared[subsystem .. "_backend_cache"]:set(backend["name"], common.json_encode(backend))
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
local _, err = ngx_timer.every(sync_backend_interval, balancer.sync_backends)
if err then
    ngx_log(ngx.ERR, string_format("error when setting up timer.every for sync_backends: %s", tostring(err)))
end
