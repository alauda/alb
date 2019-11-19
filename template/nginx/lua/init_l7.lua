local cache = require "cache"
local ngx_config = ngx.config

local subsystem = ngx_config.subsystem

cache.init_mlcache("rule_cache", subsystem .. "_alb_cache", {
    lru_size = 2000,
    ttl      = 60,
    neg_ttl  = 5,
})

cache.init_mlcache("cert_cache", subsystem .. "_alb_cache", {
    lru_size = 500,
    ttl      = 60,
    neg_ttl  = 5,
})

cache.init_mlcache("backend_cache", subsystem .. "_alb_cache", {
    lru_size = 500,
    ttl      = 60,
    neg_ttl  = 5,
})
