local balancer_resty = require "balancer.resty"
local resty_roundrobin = require "resty.roundrobin"
local bc = require "balancer.common"

local _M = balancer_resty:new({ factory = resty_roundrobin, name = "round_robin" })

function _M.new(self, backend)
    local nodes = bc.get_nodes(backend)
    local o = {
        instance = self.factory:new(nodes),
    }
    setmetatable(o, self)
    self.__index = self
    return o
end

function _M.balance(self)
    return self.instance:find()
end

return _M
