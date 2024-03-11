local common = require "utils.common"
local M = {}

function M.set_policy_json_str(policy)
    local p = require "policy_fetch"
    p.update_policy(policy, "manual")
    ngx.sleep(3)
end

function M.set_policy_lua(policy_table)
    M.set_policy_json_str(common.json_encode(policy_table, true))
end
return M
