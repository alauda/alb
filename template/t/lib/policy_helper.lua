local common = require "utils.common"
local M = {}
function M.set_policy_lua(policy_table)
    local p = require "policy_fetch"
    p.update_policy(common.json_encode(policy_table, true), "manual")
    ngx.sleep(3)
end
return M