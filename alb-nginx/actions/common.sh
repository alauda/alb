#!/bin/bash

configmap_to_file() {
  local output_dir=$1
  local configmap=$ALB/chart/templates/configmap.yaml
  echo $configmap
  sed -n '/{{-/!p' $configmap |yq  e 'select(documentIndex == 0)|.data.http' -  |sed '/access_log*/d' |sed '/keepalive*/d' > $output_dir/http.conf || true
  sed -n '/{{-/!p' $configmap |yq  e 'select(documentIndex == 0)|.data.http_server' - > $output_dir/http_server.conf || true
  sed -n '/{{-/!p' $configmap |yq  e 'select(documentIndex == 0)|.data.upstream' - > $output_dir/upstream.conf || true
  sed -n '/{{-/!p' $configmap |yq  e 'select(documentIndex == 0)|.data.stream-common' - | sed '/access_log*/d' | sed '/error_log*/d' > $output_dir/stream-common.conf || true
  sed -n '/{{-/!p' $configmap |yq  e 'select(documentIndex == 0)|.data.stream-tcp' - > $output_dir/stream-tcp.conf || true
  sed -n '/{{-/!p' $configmap |yq  e 'select(documentIndex == 0)|.data.stream-udp' - > $output_dir/stream-udp.conf || true
}

ALB_TEST_RUNNER_IMAGE=build-harbor.alauda.cn/3rdparty/alb-nginx-test:20220214184520
test-nginx() {
  local filter=""
  if [ ! -z "$1" ]
  then
    filter=$1
  fi

  docker run \
      -e TEST_NGINX_SLEEP=0.00001 \
      -e TEST_NGINX_VERBOSE=true \
      -e SYNC_POLICY_INTERVAL=1 \
      -e CLEAN_METRICS_INTERVAL=2592000 \
      -e NEW_POLICY_PATH=/usr/local/openresty/nginx/conf/policy.new \
      -v $ALB/alb-nginx/t:/t \
      -v $ALB:/alb \
      $ALB_TEST_RUNNER_IMAGE prove -I / -I /test-nginx/lib/ -r t/$filter
}

test-nginx-in-ci() {
  echo "test-nginx-in-ci" alb is $ALB
  export TEST_NGINX_VERBOSE=true
  export SYNC_POLICY_INTERVAL=1
  export CLEAN_METRICS_INTERVAL=2592000
  export NEW_POLICY_PATH=/usr/local/openresty/nginx/conf/policy.new
  export TEST_NGINX_RANDOMIZE=0
  export TEST_NGINX_SERVROOT=/t/servroot
  export TEST_NGINX_SLEEP=0.0001
  mkdir -p /t/servroot
  cp -r $ALB /alb
  rm -rf /alb/tweak || true
  cp -r /alb/alb-nginx/t/* /t
  cd /
  prove -I / -I /test-nginx/lib/ -r t
}


test-nginx-exec() {
  echo "run  'prove -I / -I /test-nginx/lib/' in this docker"
  docker run -it \
      -e TEST_NGINX_SLEEP=0.0001 \
      -e TEST_NGINX_VERBOSE=true \
      -e SYNC_POLICY_INTERVAL=1 \
      -e CLEAN_METRICS_INTERVAL=2592000 \
      -e NEW_POLICY_PATH=/usr/local/openresty/nginx/conf/policy.new \
      -v $ALB/alb-nginx/t:/t \
      -v $ALB:/alb \
      -v $ALB/chart/:/alb/chart \
      -v /tmp/alb/dhparam.pem:/etc/ssl/dhparam.pem \
      $ALB_TEST_RUNNER_IMAGE sh
}

# given a policy.new i want to run alb-nginx
function alb-nginx-run() {
	# generate nginx.conf
	# volume lua script
	echo $PWD
	mkdir -p ./alb/tweak
	configmap_to_file ./alb/tweak
	# copy from a running alb-nginx container
	local nginx_conf=$(
    cat <<'EOF'
user  root;
worker_rlimit_nofile 100000;
worker_processes     8;
worker_cpu_affinity  auto;
worker_shutdown_timeout 240s;

error_log  /var/log/nginx/error.log notice;
pid        /var/run/nginx.pid;

env SYNC_POLICY_INTERVAL;
env CLEAN_METRICS_INTERVAL;
env SYNC_BACKEND_INTERVAL;
env NEW_POLICY_PATH;
env DEFAULT-SSL-STRATEGY;
env INGRESS_HTTPS_PORT;

events {
    multi_accept        on;
    worker_connections  51200;
}

# Name: alb-dev
http {
    include       /usr/local/openresty/nginx/conf/mime.types;
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
        require("metrics").init()
    }
    init_worker_by_lua_file /alb/template/nginx/lua/worker.lua;

    server {
        listen     0.0.0.0:1936;
        listen     [::]:1936;
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
        location /policies {
            content_by_lua_file /alb/template/nginx/lua/policy.lua;
            client_body_buffer_size 5m;
            client_max_body_size 10m;
        }
    }

    server {
        listen     0.0.0.0:8080 backlog=2048 default_server;
        listen     [::]:8080 backlog=2048 default_server;

        server_name _;

        include       /alb/tweak/http_server.conf;

        location / {
            set $upstream default;
            set $rule_name "";
            set $backend_protocol http;

            rewrite_by_lua_file /alb/template/nginx/lua/l7_rewrite.lua;
            proxy_pass $backend_protocol://http_backend;
            header_filter_by_lua_file /alb/template/nginx/lua/l7_header_filter.lua;

            log_by_lua_block {
                require("metrics").log()
            }
        }
    }

    server {
        listen     0.0.0.0:8090 backlog=2048 default_server;
        listen     [::]:8090 backlog=2048 default_server;

        server_name _;

        include       /alb/tweak/http_server.conf;

        location / {
            proxy_pass http://simple_nginx_backend;
        }
    }

    server {

        listen     0.0.0.0:8443 ssl http2 backlog=2048;
        listen     [::]:8443 ssl http2 backlog=2048;

        server_name _;

        include       /alb/tweak/http_server.conf;
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
                require("metrics").log()
            }
        }
    }

    upstream simple_nginx_backend {
        server 10.0.0.222:9090;
        include       /alb/tweak/upstream.conf;
    }

    upstream http_backend {
        server 0.0.0.1:1234;   # just an invalid address as a place holder

        balancer_by_lua_block {
            balancer.balance()
        }
        include       /alb/tweak/upstream.conf;
    }
}

stream {
    include       /alb/tweak/stream-common.conf;

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
        include       /alb/tweak/stream-tcp.conf;
        listen     0.0.0.0:8081;
        listen     [::]:8081 ;
        preread_by_lua_file /alb/template/nginx/lua/l4_preread.lua;
        proxy_pass stream_backend;
    }
    server {
        include       /alb/tweak/stream-udp.conf;
        listen     0.0.0.0:8553 udp;
        listen     [::]:8553 udp;
        preread_by_lua_file /alb/template/nginx/lua/l4_preread.lua;
        proxy_pass stream_backend;
    }

    upstream stream_backend {
        server 0.0.0.1:1234;   # just an invalid address as a place holder
        balancer_by_lua_block {
            balancer.balance()
        }
    }
}
EOF
  )
    echo "$nginx_conf" > ./alb/nginx.conf
	
	local policy=$(
    cat <<EOF
{
  "certificate_map": {},
  "http": {"tcp":{
    "8080": [
            {
              "rule": "test-rule-1",
              "internal_dsl": [["STARTS_WITH","URL","/"]],
              "upstream": "test-upstream-1"
            }
    ]}
  },
  "backend_group": [
    {
      "name": "test-upstream-1",
      "mode": "http",
      "backends": [
        {
          "address": "10.0.0.222",
          "port":9090,
          "weight": 100
        }
      ]
    }
  ]
}
EOF
)
    echo "$policy" > ./alb/policy.new
	local docker_env=$(
    cat <<EOF
DEFAULT-SSL-STRATEGY=Both
INGRESS_HTTPS_PORT=443
SYNC_POLICY_INTERVAL=1
NEW_POLICY_PATH=/alb/policy.new
CLEAN_METRICS_INTERVAL=2592000
EOF
)
	echo "$docker_env" > ./docker-env
	local nginx_image="build-harbor.alauda.cn/3rdparty/alb-nginx:20220118182511"
	local nginx_image="alb-nginx:latest"
	local nginx_image="alb-nginx-ubuntu:latest"

	echo "ssl"
	openssl dhparam -dsaparam -out ./dhparam.pem 2048 
	local uid=$(id -u)
	local gid=$(id -g)
	local lua_dir=$ALB/template/nginx
	echo "run alb-nginx"

	docker run --env-file ./docker-env --network host -it  -v $PWD/log:/var/log/nginx -v $PWD/dhparam.pem:/etc/ssl/dhparam.pem -v $lua_dir:/alb/template/nginx -v $PWD/alb:/alb  $nginx_image  nginx -g "daemon off;" -c /alb/nginx.conf
	# docker run -it --env-file ./docker-env --network host -it  -v $PWD/log:/var/log/nginx -v $PWD/dhparam.pem:/etc/ssl/dhparam.pem -v $lua_dir:/alb/template/nginx -v $PWD/alb:/alb  $nginx_image sh 
	return
}

