local common = require "common"

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
  local nodes = common.get_nodes(backend)
  local changed = not common.table_equals(self.instance.nodes, nodes)
  if not changed then
    return
  end

  ngx_log(INFO, string_format("[%s] nodes have changed for backend %s", self.name, backend.name))

  self.instance:reinit(nodes)
end

return _M
