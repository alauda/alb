#!/bin/bash
set -e

_configmap_to_file() {
  local output_dir=$1
  local configmap=$ALB/chart/templates/configmap.yaml
  echo $configmap
  cat $configmap |yq  e 'select(documentIndex == 0)|.data.http' -  |sed '/access_log*/d' |sed '/keepalive*/d' > $output_dir/http.conf || true
  cat $configmap |yq  e 'select(documentIndex == 0)|.data.http_server' - > $output_dir/http_server.conf || true
  cat $configmap |yq  e 'select(documentIndex == 0)|.data.upstream' - > $output_dir/upstream.conf || true
  cat $configmap |yq  e 'select(documentIndex == 0)|.data.stream' - > $output_dir/stream.conf || true
}


test-nginx() {
  local filter=""
  if [ ! -z "$1" ]
  then
    filter=$1
  fi
  rm -rf /tmp/alb
  mkdir -p /tmp/alb/tweak
  _configmap_to_file /tmp/alb/tweak

  openssl dhparam -dsaparam -out /tmp/alb/dhparam.pem 2048

  docker run \
      -e TEST_NGINX_SLEEP=0.0001 \
      -e TEST_NGINX_VERBOSE=true \
      -e SYNC_POLICY_INTERVAL=1 \
      -e NEW_POLICY_PATH=/usr/local/openresty/nginx/conf/policy.new \
      -v $ALB/alb-nginx/t:/t \
      -v $ALB/alb-nginx/actions/:/actions \
      -v /tmp/alb/dhparam.pem:/etc/ssl/dhparam.pem \
      -v $ALB/template/nginx/:/alb/template/nginx \
      -v /tmp/alb/tweak/:/alb/tweak \
      -v /tmp/test-nginx:/test-nginx \
      -v $ALB/3rd-lua-module/lib/resty/worker:/usr/local/openresty/site/lualib/resty/worker \
      build-harbor.alauda.cn/3rdparty/alb-nginx-test:v3.6.0 prove -I / -I /test-nginx/lib/ -r t/$filter 
}

test-nginx-exec() {
  rm -rf /tmp/alb
  mkdir -p /tmp/alb/tweak
  _configmap_to_file /tmp/alb/tweak
  openssl dhparam -dsaparam -out /tmp/alb/dhparam.pem 2048

  docker run -it \
      -e TEST_NGINX_SLEEP=0.0001 \
      -e TEST_NGINX_VERBOSE=true \
      -e SYNC_POLICY_INTERVAL=1 \
      -e NEW_POLICY_PATH=/usr/local/openresty/nginx/conf/policy.new \
      -v $ALB/alb-nginx/t:/t \
      -v $ALB/alb-nginx/actions/:/actions \
      -v /tmp/alb/dhparam.pem:/etc/ssl/dhparam.pem \
      -v $ALB/template/nginx/:/alb/template/nginx \
      -v /tmp/alb/tweak/:/alb/tweak \
      -v /tmp/test-nginx:/test-nginx \
      -v $ALB/3rd-lua-module/lib/resty/worker:/usr/local/openresty/site/lualib/resty/worker \
      build-harbor.alauda.cn/3rdparty/alb-nginx-test:v3.6.0 sh
}

test-nginx-in-ci() {
  export TEST_NGINX_VERBOSE=true
  export SYNC_POLICY_INTERVAL=1
  export NEW_POLICY_PATH=/usr/local/openresty/nginx/conf/policy.new
  export TEST_NGINX_RANDOMIZE=0
  export TEST_NGINX_SERVROOT=/t/servroot
  export TEST_NGINX_SLEEP=0.0001
  cp -r $ALB/3rd-lua-module/lib/resty/worker  /usr/local/openresty/site/lualib/resty
  mkdir -p /t/servroot
  mkdir -p /alb
  cp -r $ALB/template /alb
  mkdir -p /alb/tweak
  _configmap_to_file /alb/tweak
  openssl dhparam -dsaparam -out /etc/ssl/dhparam.pem 2048
  cp ./alb-nginx/t/* /t
  cd /
  prove -I / -I /test-nginx/lib/ -r t
}
