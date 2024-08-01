local common = require "utils.common"
local p = require "config.policy_fetch"
local M = {}

function M.set_policy_json_str(policy)
    p.update_policy(policy, "manual")
    ngx.sleep(1)
end

function M.set_policy_lua(policy_table)
    M.set_policy_json_str(common.json_encode(policy_table, true))
end

return M
