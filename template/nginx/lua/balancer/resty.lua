local common = require "utils.common"
local bc = require "balancer.common"
local peer_map_helper = require "balancer.peer_map_helper"
local string_format = string.format
local ngx_log = ngx.log
local INFO = ngx.INFO

local _M = {}

function _M.new(self, o)
    o = o or {}
    setmetatable(o, self)
    self.__index = self
    return o
end

function _M.sync(self, backend)
    local nodes = bc.get_nodes(backend)
    self.backend = backend
    if self.backend.peer_map == nil then
        self.backend.peer_map = peer_map_helper.init_peer_map(backend)
    end
    local changed = not common.table_equals(self.instance.nodes, nodes)
    if not changed then
        return
    end

    ngx_log(INFO, string_format("[%s] nodes have changed for backend %s", self.name, backend.name))
    self.backend.peer_map = peer_map_helper.init_peer_map(backend)
    self.instance:reinit(nodes)
end

---
---comment
---@param self table
---@param node_key NodeKey
---@return BackendConf
function _M.get_peer_conf(self, node_key)
    local config = peer_map_helper.get_peer_conf(self.backend.peer_map, node_key)
    if not config then
        ngx.log(ngx.WARN, string_format("resty no peer config found for node_key %s", node_key))
        return {
            ns = "unknown",
            svc = "unknown",
            port = 0,
            address = "127.0.0.1",
            weight = 0,
        }
    end
    return config
end

return _M
