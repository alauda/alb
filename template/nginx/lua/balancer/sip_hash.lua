local balancer_resty = require "balancer.resty"
local resty_chash = require "resty.chash"
local common = require "utils.common"
local bc = require "balancer.common"
local sys = require("utils.subsystem")
local x_forward = require("utils.x_forward")
local _M = balancer_resty:new({ factory = resty_chash, name = "chash" })

local DISABLE_PREFER_L7_SOURCE_IP = os.getenv("DISABLE_PREFER_L7_SOURCE_IP") == "true"

function _M.new(self, backend)
    local nodes = bc.get_nodes(backend)
    local o = {
        instance = self.factory:new(nodes),
    }
    if backend["session_affinity_policy"] == "sip-hash" then
        o.type = "sip-hash"
    end
    setmetatable(o, self)
    self.__index = self
    return o
end

function _M.find_source_ip()
    local l4_source_ip = common.lua_ngx_var("$remote_addr")
    if sys.is_stream_subsystem() or DISABLE_PREFER_L7_SOURCE_IP then
        return l4_source_ip
    end
    local l7_source_ip = x_forward.parse_first_forward(common.lua_ngx_var("$http_x_forwarded_for"))
    if l7_source_ip ~= nil then
        return l7_source_ip
    end
    return l4_source_ip
end

function _M.balance(self)
    local key = _M.find_source_ip()
    return self.instance:find(key)
end

return _M
