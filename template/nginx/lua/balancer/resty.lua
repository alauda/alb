local common = require "utils.common"
local bc = require "balancer.common"
local new_tab = require "table.new"

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

local function init_peer_map(backend)
    local peer_map = new_tab(0, #backend.backends)
    for i, node in ipairs(backend.backends) do
        peer_map[bc.node_key(node)] = i
    end
    return peer_map
end

function _M.sync(self, backend)
    local nodes = bc.get_nodes(backend)
    self.backend = backend
    if self.backend.peer_map == nil then
        self.backend.peer_map = init_peer_map(backend)
    end
    local changed = not common.table_equals(self.instance.nodes, nodes)
    if not changed then
        return
    end

    ngx_log(INFO, string_format("[%s] nodes have changed for backend %s", self.name, backend.name))
    self.backend.peer_map = init_peer_map(backend)
    self.instance:reinit(nodes)
end

function _M.get_peer_conf(self, peer)
    local index = self.backend.peer_map[peer]
    return self.backend.backends[index]
end

return _M
