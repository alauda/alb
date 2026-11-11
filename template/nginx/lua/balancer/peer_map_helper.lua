local _M = {}

local bc = require "balancer.common"
local new_tab = require "table.new"
---comment
--- @param backend BackendGroup
--- @return table<NodeKey, BackendConf>
function _M.init_peer_map(backend)
    local peer_map = new_tab(0, #backend.backends)
    for _, node in ipairs(backend.backends) do
        peer_map[bc.node_key(node)] = node
    end
    return peer_map
end

---
---comment
--- @param peer_map table
--- @param node_key NodeKey
--- @return BackendConf?
function _M.get_peer_conf(peer_map, node_key)
    return peer_map[node_key]
end

return _M
