#!/bin/bash

function alb-install-nginx-test-dependency {
  apk update && apk add luarocks luacheck lua perl-app-cpanminus wget curl make build-base perl-dev git neovim bash yq jq tree fd openssl
  cpanm --mirror-only --mirror https://mirrors.tuna.tsinghua.edu.cn/CPAN/ -v --notest Test::Nginx IPC::Run
}

function alb-test-all-in-ci-nginx {
  # base image build-harbor.alauda.cn/3rdparty/alb-nginx:v3.9-57-gb40a7de
  set -e # exit on err
  echo alb is $ALB
  echo pwd is $(pwd)
  local start=$(date +"%Y %m %e %T.%6N")
  if [ -z "$SKIP_INSTALL_NGINX_TEST_DEP" ]; then
    alb-install-nginx-test-dependency
  fi
  local end_install=$(date +"%Y %m %e %T.%6N")
  #   alb-lint-lua # TODO
  local end_check=$(date +"%Y %m %e %T.%6N")
  test-nginx-in-ci
  local end_test=$(date +"%Y %m %e %T.%6N")
  echo "start " $start
  echo "install " $end_install
  echo "check" $end_check
  echo "test" $end_test
}

function test-nginx-in-ci() {
  set -e
  set -x
  echo "test-nginx-in-ci" alb is $ALB
  export TEST_NGINX_VERBOSE=true
  export SYNC_POLICY_INTERVAL=1
  export CLEAN_METRICS_INTERVAL=2592000
  export NEW_POLICY_PATH=/etc/alb2/nginx/policy.new
  export TEST_NGINX_RANDOMIZE=0
  export TEST_NGINX_SERVROOT=/t/servroot
  export TEST_NGINX_SLEEP=0.0001
  export TEST_NGINX_WORKER_USER=root
  mkdir -p /etc/alb2/nginx
  if [ ! -d /t/servroot ]; then
    mkdir -p /t/servroot
  fi
  configmap_to_file /alb/tweak
  openssl dhparam -dsaparam -out /etc/alb2/nginx/dhparam.pem 2048
  cp -r $ALB/alb-nginx/t/* /t
  tree /alb
  tree /t
  cd /
  local filter=""
  if [ ! -z "$1" ]; then
    filter=$1
  fi
  whoami
  prove -I / -I /test-nginx/lib/ -r t/$filter
}

function configmap_to_file() {
  local output_dir=$1
  mkdir -p $output_dir
  ls /alb
  /alb/tools/tweak_gen $output_dir
  ls -alh $output_dir
  sed -i '/access_log*/d' $output_dir/http.conf
  sed -i '/keepalive*/d' $output_dir/http.conf
  sed -i '/access_log*/d' $output_dir/stream-common.conf
  sed -i '/error_log*/d' $output_dir/stream-common.conf
  cat $output_dir/http.conf
}
