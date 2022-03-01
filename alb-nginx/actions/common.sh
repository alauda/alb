#!/bin/bash
set -e

configmap_to_file() {
  local output_dir=$1
  local configmap=$ALB/chart/templates/configmap.yaml
  echo $configmap
  cat $configmap |yq  e 'select(documentIndex == 0)|.data.http' -  |sed '/access_log*/d' |sed '/keepalive*/d' > $output_dir/http.conf || true
  cat $configmap |yq  e 'select(documentIndex == 0)|.data.http_server' - > $output_dir/http_server.conf || true
  cat $configmap |yq  e 'select(documentIndex == 0)|.data.upstream' - > $output_dir/upstream.conf || true
  cat $configmap |yq  e 'select(documentIndex == 0)|.data.stream-common' - | sed '/access_log*/d' | sed '/error_log*/d' > $output_dir/stream-common.conf || true
  cat $configmap |yq  e 'select(documentIndex == 0)|.data.stream-tcp' - > $output_dir/stream-tcp.conf || true
  cat $configmap |yq  e 'select(documentIndex == 0)|.data.stream-udp' - > $output_dir/stream-udp.conf || true
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
      -v $ALB/3rd-lua-module/lib/resty/worker:/usr/local/openresty/site/lualib/resty/worker \
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
  cp -r $ALB/3rd-lua-module/lib/resty/worker  /usr/local/openresty/site/lualib/resty
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
      -v $ALB/3rd-lua-module/lib/resty/worker:/usr/local/openresty/site/lualib/resty/worker \
      $ALB_TEST_RUNNER_IMAGE sh
}

