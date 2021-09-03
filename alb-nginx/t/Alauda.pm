use strict;
use warnings;

package t::Alauda;
use Test::Nginx::Socket::Lua::Stream -Base;


add_block_preprocessor(sub {
    my $block = shift;
    my $policy = $block->policy;
    my $server_port= $block->server_port;
    if (defined server_port) {
        warn "set server_port to $server_port";
        server_port_for_client($server_port);
    }

    my $no_response_code= $block->no_response_code;
    if (defined $no_response_code) {
        $block->set_value("error_code",'');
    }

    open(FH,'>','/usr/local/openresty/nginx/conf/policy.new') or die $!;
    print FH $policy;
    close(FH);

        $block->set_value("main_config", <<'_END_');
env SYNC_POLICY_INTERVAL;
env SYNC_BACKEND_INTERVAL;
env NEW_POLICY_PATH;
env DEFAULT-SSL-STRATEGY;
env INGRESS_HTTPS_PORT;

stream {
    include       /alb/tweak/stream.conf;

    lua_add_variable $upstream;

    init_by_lua_block {
            require "resty.core"
            ok, res = pcall(require, "balancer")
            if not ok then
              error("require failed: " .. tostring(res))
            else
              balancer = res
            end
    }
    init_worker_by_lua_file /alb/template/nginx/lua/worker.lua;

    server {
        listen     0.0.0.0:1985;
        preread_by_lua_file /alb/template/nginx/lua/l4_preread.lua;
        proxy_pass tcp_backend;
    }
    
    upstream tcp_backend {
        server 0.0.0.1:1234;   # just an invalid address as a place holder

        balancer_by_lua_block {
            balancer.balance()
        }
    }
}
_END_

my $http_config = $block->http_config;
        $block->set_value("http_config", <<_END_);
include       /alb/tweak/http.conf;

init_by_lua_block {
    require "resty.core"
    ok, res = pcall(require, "balancer")
        if not ok then
            error("require failed: " .. tostring(res))
    else
        balancer = res
    end
    -- require("metrics").init()
}
$http_config
init_worker_by_lua_file /alb/template/nginx/lua/worker.lua;
upstream http_backend {
    server 0.0.0.1:1234;   # just an invalid address as a place holder

    balancer_by_lua_block {
        balancer.balance()
    }
        include       /alb/tweak/upstream.conf;
}
_END_

        $block->set_value("config", <<'_END_');
include       /alb/tweak/http_server.conf;
access_log /t/servroot/logs/access.log http;
add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;

ssl_certificate /alb/template/nginx/placeholder.crt;
ssl_certificate_key /alb/template/nginx/placeholder.key;

ssl_certificate_by_lua_file /alb/template/nginx/lua/cert.lua;

location / {
    set $upstream default;
    set $rule_name "";
    set $backend_protocol http;

    rewrite_by_lua_file /alb/template/nginx/lua/l7_rewrite.lua;
    proxy_pass $backend_protocol://http_backend;
    header_filter_by_lua_file /alb/template/nginx/lua/l7_header_filter.lua;

    log_by_lua_block {
      --  require("metrics").log()
    }
}
_END_
});

return 1;