use strict;
use warnings;

package t::Alauda;
use Test::Nginx::Socket::Lua::Stream -Base;

# struct of a nginx test
# /
# /nginx
#     /lua
#     /placeholder.cert
#     /placeholder.key
# /t
# /nginx.conf
# /tweak
# /logs
# /dhparam.pem
# /policy.new

# to knowing how/why those $block->set_value work, take a look at https://github.com/openresty/test-nginx/blob/be75f595236eef83e4274363e13affdf08b05737/lib/Test/Nginx/Util.pm#L968  

sub gen_https_port_config {
    my $ports = shift; 
    my $base = shift; 
    my @port_list = split(',', $ports);
    my $result = '';
    warn "ports is $ports\n";
    warn "base is $base\n";
    for (my $i = 0; $i < scalar @port_list; $i++) {
        my $port = $port_list[$i];
        $result .= <<__END;
    server {
        listen     0.0.0.0:$port ssl http2 backlog=2048;
        listen     [::]:$port ssl http2 backlog=2048;

        server_name _;

        include       $base/tweak/http_server.conf;
        add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;

        ssl_certificate $base/nginx/placeholder.crt;
        ssl_certificate_key $base/nginx/placeholder.key;
        ssl_certificate_by_lua_file $base/nginx/lua/cert.lua;
        ssl_dhparam $base/dhparam.pem;

        location / {
            set \$backend_protocol http;

            rewrite_by_lua_file $base/nginx/lua/l7_rewrite.lua;
            proxy_pass \$backend_protocol://http_backend;
            header_filter_by_lua_file $base/nginx/lua/l7_header_filter.lua;

            log_by_lua_block {
                require("metrics").log()
            }
        }
    }
__END
    }

    return $result;
}

add_block_preprocessor(sub {
    my $block = shift;
    my $base = $ENV{'TEST_BASE'};
    my $alb = $ENV{'TEST_BASE'}; 
    warn "base is $base";
    my $lua_path= "/usr/local/lib/lua/?.lua;$base/nginx/lua/?.lua;$base/t/?.lua;$base/t/lib/?.lua;$base/nginx/lua/vendor/?.lua;;";
    my $server_port = $block->server_port;
    system("mkdir -p $base/logs");
    my $no_response_code= $block->no_response_code;
    if (defined $no_response_code) {
        $block->set_value("error_code",'');
    }

    unless (-e "$base/cert/tls.key") {
        my $cmd="openssl req -x509 -newkey rsa:4096 -sha256 -days 3650 -nodes -keyout $base/cert/tls.key -out  $base/cert/tls.crt -subj \"/CN=test.com\"";
        warn "cmd $cmd";
        system($cmd);
    }
    my $certificate= $block->certificate;
    if (defined $certificate) {
        my @certs = split /\s+/, $certificate;
        my $crt = $certs[0];
        my $key = $certs[1];
        my $cmd="openssl req -x509 -newkey rsa:4096 -sha256 -days 3650 -nodes -keyout $key -out $crt -subj \"/CN=test.com\"";
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
    my $lua_test_mode = "false";
    my $lua_test_full = '';
    if (defined  $block->lua_test_file) {
        $server_port = 1999;
        $lua_test_mode = "true";
		my $lua_test_file=$block->lua_test_file;
        $lua_test_full = <<__END;
server {
    listen 1999;
    location /t {
        content_by_lua_block {
			local test=function()
				require("$lua_test_file").test()
			end
			test()
			ngx.print("ok")
		}
	}
}
__END
    }

    if (defined  $block->lua_test) {
		my $lua_test=$block->lua_test;
        $server_port = 1999;
        $lua_test_mode = "true";
        $lua_test_full = <<__END;
server {
    listen 1999;
    location /t {
        content_by_lua_block {
			local test=function()
				$lua_test
			end
			test()
			ngx.print("ok")
		}
	}
}
__END
	}
    if (defined  $block->lua_test_eval) {
        $server_port = 1999;
        $lua_test_mode = "true";
		my $lua_test_eval=$block->lua_test_eval;
        $lua_test_full = <<__END;
server {
    listen 1999;
    location /t {
        content_by_lua_block {
			local test = function()
				$lua_test_eval
			end
			local ok,ret = pcall(test)
            if not ok then
                ngx.log(ngx.ERR," sth wrong "..tostring(ret).."  "..tostring(ok))
			    ngx.print("fail")
                ngx.exit(ngx.ERROR)
            end
			ngx.print("ok")
		}
	}
}
__END
	}else {
        warn "lua_test_eval not defined"
    }

    if (defined $server_port) {
        warn "set server_port to $server_port";
        server_port_for_client($server_port);
    }else {
        warn "server_port not defined";
    }
    if (!defined $block->request) {
        $block->set_value("request","GET /t");
    }

   	if (!defined  $block->response_body and not $lua_test_mode eq "true" and not defined $block->response) {
        warn "response_body ok";
		$block->set_value("response_body","ok");
	}


    open(FH,'>',"$base/policy.new") or die $!;
    print FH $policy;
    close(FH);
    

	my $init_worker = " init_worker_by_lua_file $base/nginx/lua/worker.lua; ";
    if (defined $block->disable_init_worker) {
        $init_worker = "";
    }

    if (defined $block->init_worker_eval) {
        my $init_worker_eval=$block->init_worker_eval;
        my $init_worker_lua = <<__END;
init_worker_by_lua_block {
    $init_worker_eval
}
__END
        $init_worker = $init_worker_lua;
    }

    $block->set_value("main_config", <<_END_);
env SYNC_POLICY_INTERVAL;
env CLEAN_METRICS_INTERVAL;
env SYNC_BACKEND_INTERVAL;
env NEW_POLICY_PATH;
env DEFAULT_SSL_STRATEGY;
env DEFAULT_SSL_STRATEGY;
env INGRESS_HTTPS_PORT;
env TEST_BASE;

stream {
    include       $base/tweak/stream-common.conf;
    lua_package_path "$lua_path";

    access_log $base/servroot/logs/access.stream.log stream;
    error_log $base/servroot/logs/error.stream.log info;

    init_by_lua_block {
            require "resty.core"
            ok, res = pcall(require, "balancer")
            if not ok then
                error("require failed: " .. tostring(res))
            else
                balancer = res
            end
    }
    $init_worker
    $stream_config

    server {
        include       $base/tweak/stream-tcp.conf;
        listen     0.0.0.0:81;
        preread_by_lua_file $base/nginx/lua/l4_preread.lua;
        proxy_pass stream_backend;
    }
    
    server {
        include       $base/tweak/stream-udp.conf;
        listen     0.0.0.0:82 udp;
        preread_by_lua_file $base/nginx/lua/l4_preread.lua;
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

    my $extra_https_port_config = "";

    if (defined $block->alb_https_port) {
        $extra_https_port_config = gen_https_port_config("",$block->alb_https_port,$base);
    }


    if (defined $block->mock_backend) {
        my $mock_backend = $block->mock_backend;
        my @array = split ' ', $mock_backend;
        my $port = $array[0];
        my $module = $array[1];
        warn "get mock backend $port | $module";
        my $cfg= <<__END;
server {
    listen $port;
    location / {
       content_by_lua_block {
            require("$module").as_backend($port)
      }
    }
}
__END
        $block->set_value("http_config",$cfg);
    }

    my $http_config;
    if (defined $block->http_config) {
        $http_config = $block->http_config;
    }else {
        $http_config = "";
    }
    # warn "get http config $http_config";

    my $cfg = <<__END;
    include       $base/tweak/http.conf;
    lua_package_path "$lua_path";
    error_log $base/servroot/logs/error.http.log info;

	log_format  test  '[\$time_local] \$remote_addr:\$remote_port "\$host" "\$request" '
                      '\$status \$upstream_status \$upstream_addr '
                      '"\$http_user_agent" "\$http_x_forwarded_for" '
                      '\$request_time \$upstream_response_time';
    access_log $base/servroot/logs/access.http.log test;

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
            require("metrics").init()
    }

    $init_worker

    server {
        listen     0.0.0.0:28080 backlog=2048 default_server;
        listen     [::]:28080 backlog=2048 default_server;
        server_name _;
        include       $base/tweak/http_server.conf;
        location / {
            set \$backend_protocol http;

            rewrite_by_lua_file $base/nginx/lua/l7_rewrite.lua;
            proxy_pass \$backend_protocol://http_backend;
            header_filter_by_lua_file $base/nginx/lua/l7_header_filter.lua;

            log_by_lua_block {
                require("metrics").log()
            }
        }
    }

    server {
        listen     0.0.0.0:80 backlog=2048 default_server;
        listen     [::]:80 backlog=2048 default_server;
        server_name _;
        include       $base/tweak/http_server.conf;
        location / {
            set \$backend_protocol http;
 
            rewrite_by_lua_file $base/nginx/lua/l7_rewrite.lua;
            proxy_pass \$backend_protocol://http_backend;
            header_filter_by_lua_file $base/nginx/lua/l7_header_filter.lua;

            log_by_lua_block {
                require("metrics").log()
            }
        }
    }

    server {
        listen     0.0.0.0:3443 ssl backlog=2048;
        listen     [::]:3443 ssl backlog=2048;

        server_name _;

        include       $base/tweak/http_server.conf;
        add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;

        ssl_certificate $base/nginx/placeholder.crt;
        ssl_certificate_key $base/nginx/placeholder.key;
        ssl_certificate_by_lua_file $base/nginx/lua/cert.lua;
        ssl_dhparam $base/dhparam.pem;

        location / {
            set \$backend_protocol http;

            rewrite_by_lua_file $base/nginx/lua/l7_rewrite.lua;
            proxy_pass \$backend_protocol://http_backend;
            header_filter_by_lua_file $base/nginx/lua/l7_header_filter.lua;

            log_by_lua_block {
                require("metrics").log()
            }
        }
    }
    server {
        listen     0.0.0.0:443 ssl http2 backlog=2048;
        listen     [::]:443 ssl http2 backlog=2048;

        server_name _;

        include       $base/tweak/http_server.conf;
        add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;

        ssl_certificate $base/nginx/placeholder.crt;
        ssl_certificate_key $base/nginx/placeholder.key;
        ssl_certificate_by_lua_file $base/nginx/lua/cert.lua;
        ssl_dhparam $base/dhparam.pem;

        location / {
            set \$backend_protocol http;

            rewrite_by_lua_file $base/nginx/lua/l7_rewrite.lua;
            proxy_pass \$backend_protocol://http_backend;
            header_filter_by_lua_file $base/nginx/lua/l7_header_filter.lua;

            log_by_lua_block {
                require("metrics").log()
            }
        }
    }

    server {
        listen     0.0.0.0:2443 ssl http2 backlog=2048;
        listen     [::]:2443 ssl http2 backlog=2048;

        server_name _;

        include       $base/tweak/http_server.conf;
        add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;

        ssl_certificate $base/nginx/placeholder.crt;
        ssl_certificate_key $base/nginx/placeholder.key;
        ssl_certificate_by_lua_file $base/nginx/lua/cert.lua;
        ssl_dhparam $base/dhparam.pem;

        location / {
            set \$backend_protocol http;

            rewrite_by_lua_file $base/nginx/lua/l7_rewrite.lua;
            proxy_pass \$backend_protocol://http_backend;
            header_filter_by_lua_file $base/nginx/lua/l7_header_filter.lua;

            log_by_lua_block {
                require("metrics").log()
            }
        }
    }
    $extra_https_port_config

    upstream http_backend {
        server 0.0.0.1:1234;   # just an invalid address as a place holder

        balancer_by_lua_block {
            balancer.balance()
        }
        include       $base/tweak/upstream.conf;
    }

    $http_config

	$lua_test_full

    server {
        listen    0.0.0.0:1936;
        access_log off;

        location /status {
            stub_status;
        }

        location /metrics {
            content_by_lua_block {
                require("metrics").collect()
            }
        }

        location /clear {
            content_by_lua_block {
                require("metrics").clear()
            }
        }
    }

__END
    $block->set_value("http_config",$cfg);
});

return 1;