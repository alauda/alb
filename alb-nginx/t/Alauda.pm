use strict;
use warnings;

package t::Alauda;
use Test::Nginx::Socket::Lua::Stream -Base;


add_block_preprocessor(sub {
    warn "generate tweak conf";
    system("bash -c 'source /alb/alb-nginx/actions/common.sh && mkdir -p /alb/tweak && ALB=/alb configmap_to_file /alb/tweak'");

    warn "generate dhparam.pem";
    if (! -f  "/etc/ssl/dhparam.pem") {
        system("openssl dhparam -dsaparam -out /etc/ssl/dhparam.pem 2048");
    }

    my $block = shift;
    my $server_port= $block->server_port;
    if (defined $server_port) {
        warn "set server_port to $server_port";
        server_port_for_client($server_port);
    }

    my $no_response_code= $block->no_response_code;
    if (defined $no_response_code) {
        $block->set_value("error_code",'');
    }

    my $certificate= $block->certificate;
    if (defined $certificate) {
        my @certs = split /\s+/, $certificate;
        my $crt = $certs[0];
        my $key = $certs[1];
        my $cmd="mkdir -p /cert && openssl req -x509 -newkey rsa:4096 -sha256 -days 3650 -nodes -keyout $key -out $crt -subj \"/CN=test.com\"";
        warn "generate certificate crt $crt key $key";
        warn "cmd $cmd";
        system($cmd);
    }

    my $policy = $block->policy;
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
        listen     0.0.0.0:81;
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


    my $config = $block->config;
    if (defined $config) {
        $block->set_value("config",$config);
    }else {
        $block->set_value("config","");
    }


    my $http_config = "";
    if (defined $block->http_config) {
        $http_config = $block->http_config;
    }

    my $cfg = <<__END;
    include       /alb/tweak/http.conf;

    gzip on;
    gzip_comp_level 5;
    gzip_http_version 1.1;
    gzip_min_length 256;
    gzip_types application/atom+xml application/javascript application/x-javascript application/json application/rss+xml application/vnd.ms-fontobject application/x-font-ttf application/x-web-app-manifest+json application/xhtml+xml application/xml font/opentype image/svg+xml image/x-icon text/css text/javascript text/plain text/x-component;
    gzip_proxied any;
    gzip_vary on;

    init_by_lua_block {
            require "resty.core"
            ok, res = pcall(require, "balancer")
            if not ok then
                error("require failed: " .. tostring(res))
            else
                balancer = res
            end
            --require("metrics").init()
    }
    init_worker_by_lua_file /alb/template/nginx/lua/worker.lua;

    server {
        listen     0.0.0.0:80 backlog=2048 default_server;
        listen     [::]:80 backlog=2048 default_server;

        server_name _;

        include       /alb/tweak/http_server.conf;
        access_log /t/servroot/logs/access.log http;

        location / {
            set \$upstream default;
            set \$rule_name "";
            set \$backend_protocol http;

            rewrite_by_lua_file /alb/template/nginx/lua/l7_rewrite.lua;
            proxy_pass \$backend_protocol://http_backend;
            header_filter_by_lua_file /alb/template/nginx/lua/l7_header_filter.lua;


            log_by_lua_block {
                --require("metrics").log()
            }
        }
    }

    server {
        listen     0.0.0.0:443 ssl http2 backlog=2048;
        listen     [::]:443 ssl http2 backlog=2048;

        server_name _;

        include       /alb/tweak/http_server.conf;
        add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;

        ssl_certificate /alb/template/nginx/placeholder.crt;
        ssl_certificate_key /alb/template/nginx/placeholder.key;
        ssl_certificate_by_lua_file /alb/template/nginx/lua/cert.lua;

        location / {
            set \$upstream default;
            set \$rule_name "";
            set \$backend_protocol http;

            rewrite_by_lua_file /alb/template/nginx/lua/l7_rewrite.lua;
            proxy_pass \$backend_protocol://http_backend;
            header_filter_by_lua_file /alb/template/nginx/lua/l7_header_filter.lua;

            log_by_lua_block {
                --require("metrics").log()
            }
        }
    }

    upstream http_backend {
        server 0.0.0.1:1234;   # just an invalid address as a place holder

        balancer_by_lua_block {
            balancer.balance()
        }
        include       /alb/tweak/upstream.conf;
    }

    $http_config
__END
    $block->set_value("http_config",$cfg);
});

return 1;