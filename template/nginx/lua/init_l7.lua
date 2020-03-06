local cache = require "cache"
local ngx_config = ngx.config
local subsystem = ngx_config.subsystem

cache.init_mlcache("rule_cache", subsystem .. "_alb_cache", {
    lru_size = 2000,
    ttl      = 60,
    neg_ttl  = 5,
    ipc_shm  = subsystem .. "_ipc_shared_dict",
})

cache.init_mlcache("cert_cache", subsystem .. "_alb_cache", {
    lru_size = 500,
    ttl      = 60,
    neg_ttl  = 5,
    ipc_shm  = subsystem .. "_ipc_shared_dict",
})
