local balancer_resty = require("balancer.resty")
local ck = require("resty.cookie")
local ngx = ngx
local common = require("utils.common")

local _M = balancer_resty:new()
local DEFAULT_COOKIE_NAME = "JSESSIONID"

function _M.cookie_name(self)
    if self.session_affinity_attribute and self.session_affinity_attribute ~= "" then
        return self.session_affinity_attribute
    else
        return DEFAULT_COOKIE_NAME
    end
end

function _M.header_name(self)
    if self.session_affinity_attribute and self.session_affinity_attribute ~= "" then
        return self.session_affinity_attribute
    end
end

function _M.new(self)
    local o = {session_affinity_attribute = nil, session_affinity_policy = nil}

    setmetatable(o, self)
    self.__index = self

    return o
end

function _M.is_sticky_cookie(self)
    return self.session_affinity_policy == "cookie"
end

function _M.is_sticky_header(self)
    return self.session_affinity_policy == "header"
end

function _M.get_key(self)
    if self:is_sticky_cookie() then
        local cookie, err = ck:new()
        if not cookie then ngx.log(ngx.ERR, err) end
        return cookie:get(self:cookie_name())
    elseif self:is_sticky_header() then
        return ngx.req.get_headers()[self:header_name()]
    else
        ngx.log(ngx.ERR, "[balancer] unknow sticky policy " ..
                    tostring(self.session_affinity_policy))
        return ngx.exit(ngx.ERROR)
    end
end

function _M.set_cookie(self, value)
    local cookie, err = ck:new()
    if not cookie then ngx.log(ngx.ERR, err) end

    local cookie_data = {
        key = self:cookie_name(),
        value = value,
        path = "/",
        httponly = true,
        secure = ngx.var.https == "on"
    }

    local ok
    ok, err = cookie:set(cookie_data)
    if not ok then ngx.log(ngx.ERR, err) end
end

local function get_failed_upstreams()
    local indexed_upstream_addrs = {}
    local upstream_addrs = common.split_upstream_var(ngx.var.upstream_addr) or
                               {}

    for _, addr in ipairs(upstream_addrs) do
        indexed_upstream_addrs[addr] = true
    end

    return indexed_upstream_addrs
end

local function should_set_cookie() return true end

function _M.balance(self)
    local upstream_from_key

    local key = self:get_key()
    if key then upstream_from_key = self.instance:find(key) end
    local should_pick_new_upstream = upstream_from_key == nil

    if not should_pick_new_upstream then return upstream_from_key end

    local new_upstream,key = self:pick_new_upstream(get_failed_upstreams())
    if not new_upstream then
        ngx.log(ngx.WARN, string.format(
                    "failed to get new upstream; using upstream %s",
                    new_upstream))
    elseif self:is_sticky_cookie() and should_set_cookie() then
        self:set_cookie(key)
    end

    return new_upstream
end

function _M.sync(self, backend)
    -- reload balancer nodes
    balancer_resty.sync(self, backend)

    self.session_affinity_attribute = backend.session_affinity_attribute
    self.session_affinity_policy = backend.session_affinity_policy
end

return _M
