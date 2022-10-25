use strict;
use warnings;

package t::Alauda;
use Test::Nginx::Socket::Lua::Stream -Base;

# to knowing how/why those $block->set_value work, take a look at https://github.com/openresty/test-nginx/blob/be75f595236eef83e4274363e13affdf08b05737/lib/Test/Nginx/Util.pm#L968  
add_block_preprocessor(sub {
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

    my $stream_config= $block->alb_stream_server_config;

    if (!defined $stream_config) {
       $stream_config = <<__END;
__END
    }
    my $policy = $block->policy;

    if (!defined $policy) {
        my $defaultPolicy = <<__END;
        {
            "certificate_map": {},
            "http": {},
            "backend_group":[]
        }
__END
        $policy=$defaultPolicy;
    }
	# warn "policy is $policy";

    my $lua_test_full = '';
    if (defined  $block->lua_test) {
		my $lua_test=$block->lua_test;
    	my $server_port=1999;
        $lua_test_full = <<__END;
server {
    listen 1999;
    location /t {
        content_by_lua_block {
            package.path = '/t/?.lua;'.."/t/lib/?.lua;" .. package.path;
			local test=function()
				$lua_test
			end
			test()
			ngx.print("ok")
		}
	}
}
__END
        warn "set server_port to $server_port";
        server_port_for_client($server_port);
        $block->set_value("request","GET /t");

    	if (!defined  $block->response_body) {
			$block->set_value("response_body","ok");
		}
	}

    open(FH,'>','/etc/alb2/nginx/policy.new') or die $!;
    print FH $policy;
    close(FH);

    $block->set_value("main_config", <<_END_);
env SYNC_POLICY_INTERVAL;
env CLEAN_METRICS_INTERVAL;
env SYNC_BACKEND_INTERVAL;
env NEW_POLICY_PATH;
env DEFAULT-SSL-STRATEGY;
env INGRESS_HTTPS_PORT;

stream {
    include       /alb/tweak/stream-common.conf;

    access_log /t/servroot/logs/access.log stream;
    error_log /t/servroot/logs/error.log info;


    lua_add_variable \$upstream;

    init_by_lua_block {
            require "resty.core"
            ok, res = pcall(require, "balancer")
            if not ok then
                error("require failed: " .. tostring(res))
            else
                balancer = res
            end
    }
    init_worker_by_lua_file /alb/nginx/lua/worker.lua;

    $stream_config

    server {
        include       /alb/tweak/stream-tcp.conf;
        listen     0.0.0.0:81;
        preread_by_lua_file /alb/nginx/lua/l4_preread.lua;
        proxy_pass stream_backend;
    }
    
    server {
        include       /alb/tweak/stream-udp.conf;
        listen     0.0.0.0:82 udp;
        preread_by_lua_file /alb/nginx/lua/l4_preread.lua;
        proxy_pass stream_backend;
    }

    upstream stream_backend {
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


    my $http_config = $block->http_config;
    # warn "get http config $http_config";

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
    init_worker_by_lua_file /alb/nginx/lua/worker.lua;

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

            rewrite_by_lua_file /alb/nginx/lua/l7_rewrite.lua;
            proxy_pass \$backend_protocol://http_backend;
            header_filter_by_lua_file /alb/nginx/lua/l7_header_filter.lua;


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

        ssl_certificate /alb/nginx/placeholder.crt;
        ssl_certificate_key /alb/nginx/placeholder.key;
        ssl_certificate_by_lua_file /alb/nginx/lua/cert.lua;
        ssl_dhparam /etc/alb2/nginx/dhparam.pem;

        location / {
            set \$upstream default;
            set \$rule_name "";
            set \$backend_protocol http;

            rewrite_by_lua_file /alb/nginx/lua/l7_rewrite.lua;
            proxy_pass \$backend_protocol://http_backend;
            header_filter_by_lua_file /alb/nginx/lua/l7_header_filter.lua;

            log_by_lua_block {
                --require("metrics").log()
            }
        }
    }

    server {
        listen     0.0.0.0:2443 ssl http2 backlog=2048;
        listen     [::]:2443 ssl http2 backlog=2048;

        server_name _;

        include       /alb/tweak/http_server.conf;
        add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;

        ssl_certificate /alb/nginx/placeholder.crt;
        ssl_certificate_key /alb/nginx/placeholder.key;
        ssl_certificate_by_lua_file /alb/nginx/lua/cert.lua;

        location / {
            set \$upstream default;
            set \$rule_name "";
            set \$backend_protocol http;

            rewrite_by_lua_file /alb/nginx/lua/l7_rewrite.lua;
            proxy_pass \$backend_protocol://http_backend;
            header_filter_by_lua_file /alb/nginx/lua/l7_header_filter.lua;

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

	$lua_test_full
__END
    $block->set_value("http_config",$cfg);
});

return 1;