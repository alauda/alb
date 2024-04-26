#!/bin/bash
source ./template/actions/dev.actions.sh
source ./scripts/alb-lint-actions.sh

function alb-install-nginx-test-dependency() {
  apk update && apk add luarocks luacheck lua perl-app-cpanminus wget curl make build-base perl-dev git neovim bash yq jq tree fd openssl
  cpanm --mirror-only --mirror https://mirrors.tuna.tsinghua.edu.cn/CPAN/ -v --notest Test::Nginx IPC::Run
}

function alb-install-nginx-test-dependency-ubuntu() {
  sudo cpanm --mirror-only --mirror https://mirrors.tuna.tsinghua.edu.cn/CPAN/ -v --notest Test::Nginx IPC::Run
}

function alb-install-nginx-test-dependency-arch() {
  yay -S luarocks luacheck lua51 perl-app-cpanminus wget curl make base-devel cpanminus perl git neovim bash yq jq tree fd openssl
  # export PATH=/opt/openresty/bin:$PATH
  # export PATH=/opt/openresty/nginx/sbin:$PATH
  # init the openresty
  cpanm --mirror-only --mirror https://mirrors.tuna.tsinghua.edu.cn/CPAN/ -v --notest Test::Nginx IPC::Run
}

function alb-nginx-install-dependency() (
  # keep same as dockerfile.full
  set -x
  LUA_VAR_NGINX_MODULE_VERSION="0.5.2"
  LUA_RESTY_BALANCER_VERSION="0.04"
  openresty=/opt/openresty
  export PATH=$openresty/bin:$PATH
  opm install thibaultcha/lua-resty-mlcache
  opm install xiangnanscu/lua-resty-cookie

  (
    curl -fSL https://github.com/openresty/lua-resty-balancer/archive/v${LUA_RESTY_BALANCER_VERSION}.tar.gz -o lua-resty-balancer-v${LUA_RESTY_BALANCER_VERSION}.tar.gz
    tar xzf lua-resty-balancer-v${LUA_RESTY_BALANCER_VERSION}.tar.gz && rm -rf lua-resty-balancer-v${LUA_RESTY_BALANCER_VERSION}.tar.gz
    cd lua-resty-balancer-${LUA_RESTY_BALANCER_VERSION}
    export LUA_LIB_DIR=$openresty/lualib
    make && make install
    cd -
    sudo rm -rf ./lua-resty-balancer-${LUA_RESTY_BALANCER_VERSION}
  )
  (
    curl -fSL https://github.com/api7/lua-var-nginx-module/archive/v${LUA_VAR_NGINX_MODULE_VERSION}.tar.gz -o lua-var-nginx-module-v${LUA_VAR_NGINX_MODULE_VERSION}.tar.gz
    tar xzf lua-var-nginx-module-v${LUA_VAR_NGINX_MODULE_VERSION}.tar.gz
    rm -rf lua-var-nginx-module-v${LUA_VAR_NGINX_MODULE_VERSION}.tar.gz
    cd lua-var-nginx-module-${LUA_VAR_NGINX_MODULE_VERSION}
    ls lib/resty/*
    cp -r lib/resty/* $openresty/lualib/resty
    cd -
    sudo rm -rf ./lua-var-nginx-module-${LUA_VAR_NGINX_MODULE_VERSION}
  )
)

function tweak_gen_install() {
  go build -v -v -o ./bin/tools/tweak_gen alauda.io/alb2/cmd/utils/tweak_gen
  md5sum ./bin/tools/tweak_gen
  cp ./bin/tools/tweak_gen /usr/local/bin
}

function alb-test-all-in-ci-nginx() {
  # base image build-harbor.alauda.cn/3rdparty/alb-nginx:v3.9-57-gb40a7de
  set -e # exit on err
  echo alb is $ALB
  export PATH=$PATH:/alb/tools/
  which tweak_gen
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

function test-nginx-local() {
  #   test-nginx-in-ci $PWD/template/t/ping.t # run special test
  #   test-nginx-in-ci                        # run all test
  # make sure tweak_gen is in path
  # clean up if you use run-like-ci-nginx.sh before
  sudo rm -rf ./template/logs
  sudo rm -rf ./template/cert
  sudo rm -rf ./template/servroot
  sudo rm -rf ./template/dhparam.pem
  sudo rm -rf ./template/policy.new

  # sudo setcap CAP_NET_BIND_SERVICE=+eip `which nginx`
  local t="ping"
  if [ -n "$1" ]; then
    t="$1"
  fi
  test-nginx-in-ci $PWD/template/t/$t.t
}

function test-nginx-in-ci() (
  set -x
  set -e

  echo "test-nginx-in-ci" alb is $ALB
  alb-lint-lua
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

  export TEST_BASE=$ALB/template
  export TEST_NGINX_VERBOSE=true
  export SYNC_POLICY_INTERVAL=1
  export CLEAN_METRICS_INTERVAL=2592000
  export NEW_POLICY_PATH=$TEST_BASE/policy.new
  export TEST_NGINX_RANDOMIZE=0
  export TEST_NGINX_SERVROOT=$TEST_BASE/servroot
  export TEST_NGINX_SLEEP=0.0001
  export TEST_NGINX_WORKER_USER=root
  export DEFAULT_SSL_STRATEGY=Always
  export INGRESS_HTTPS_PORT=443

  mkdir -p $TEST_BASE/cert
  if [[ "$KEEP_TWEAK" != "true" ]]; then
    rm -rf ./template/tweak
    configmap_to_file $TEST_BASE/tweak
  fi
  openssl dhparam -dsaparam -out $TEST_BASE/dhparam.pem 2048
  local filter=""
  if [ -z "$1" ]; then
    filter="$TEST_BASE/t"
  else
    filter=$1
  fi
  ls $TEST_BASE
  unset http_proxy
  unset https_proxy
  prove -I $TEST_BASE/ -r $filter
)

function configmap_to_file() {
  local output_dir=$1
  mkdir -p $output_dir
  tweak_gen $output_dir
  ls -alh $output_dir
  sed -i '/access_log*/d' $output_dir/http.conf
  sed -i '/error_log*/d' $output_dir/http.conf
  # remove keepalive_timeout which duplicates with  https://github.com/openresty/test-nginx/blob/8d85d7197f973713b0efd418b08efb9c640c6782/lib/Test/Nginx/Util.pm#L1146
  sed -i '/keepalive*/d' $output_dir/http.conf
  # remove lua package path set in our own
  sed -i '/lua_package_path*/d' $output_dir/http.conf
  sed -i '/access_log*/d' $output_dir/stream-common.conf
  sed -i '/error_log*/d' $output_dir/stream-common.conf
  # remove lua package path set in our own
  sed -i '/lua_package_path*/d' $output_dir/stream-common.conf

  cat $output_dir/http.conf
}
