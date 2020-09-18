local balancer_resty = require "balancer.resty"
local resty_chash = require "resty.chash"
local common = require "utils.common"

local _M = balancer_resty:new({ factory = resty_chash, name = "chash" })

function _M.new(self, backend)
  local nodes = common.get_nodes(backend)
  local o = {
    instance = self.factory:new(nodes),
  }
  if backend["session_affinity_policy"] == "sip-hash" then
    o.hash_by = "remote_addr"
  end
  setmetatable(o, self)
  self.__index = self
  return o
end

function _M.balance(self)
  local key = common.lua_ngx_var(self.hash_by)
  return self.instance:find(key)
end

return _M
