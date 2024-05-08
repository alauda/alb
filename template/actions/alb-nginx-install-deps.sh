#!/bin/bash

set -x
set -e

LUA_VAR_NGINX_MODULE_VERSION="0.5.2"
LUA_RESTY_BALANCER_VERSION="0.04"
LUA_RESTY_MLCACHE_VERSION="2.5.0"
LUA_RESTY_COOKIE_VERSION="0.01"

openresty=$1
if [[ -z "$openresty" && -f "/usr/local/openresty" ]]; then
  openresty="/usr/local/openresty"
  echo "use default $openresty "
fi
if [[ -z "$openresty" && -f "/opt/openresty" ]]; then
  openresty="/opt/openresty"
  echo "use default $openresty "
fi

if [ -z "$openresty" ]; then
  echo "Usage: $0 /path/to/openresty"
  exit 1
fi

export PATH=$openresty/bin:$PATH

function install-online() (
  tree $openresty/site/
  opm install thibaultcha/lua-resty-mlcache=$LUA_RESTY_MLCACHE_VERSION
  tree $openresty/site/
  opm install xiangnanscu/lua-resty-cookie=$LUA_RESTY_COOKIE_VERSION
  tree $openresty/site/

  install-lua-var-nginx-module "https://github.com/api7/lua-var-nginx-module/archive/v${LUA_VAR_NGINX_MODULE_VERSION}.tar.gz" ${LUA_VAR_NGINX_MODULE_VERSION}
  install-lua-resty-balancer "https://github.com/openresty/lua-resty-balancer/archive/v${LUA_RESTY_BALANCER_VERSION}.tar.gz" ${LUA_RESTY_BALANCER_VERSION}
  tree $openresty/lualib/resty
)

function install-lua-resty-balancer() (
  local url="$1"
  local ver="$2"
  wget "$url" -O lua-resty-balancer-v$ver.tar.gz
  tar xzf lua-resty-balancer-v$ver.tar.gz && rm -rf lua-resty-balancer-v$ver.tar.gz
  cd lua-resty-balancer-$ver
  export LUA_LIB_DIR=$openresty/lualib
  make && make install
  cd -
  rm -rf ./lua-resty-balancer-$ver
  return
)

function install-lua-var-nginx-module() (
  local url="$1"
  local ver="$2"
  wget $url -O lua-var-nginx-module-v$ver.tar.gz
  tar xzf lua-var-nginx-module-v$ver.tar.gz
  rm -rf lua-var-nginx-module-v$ver.tar.gz
  cd lua-var-nginx-module-$ver
  ls lib/resty/*
  cp -r lib/resty/* $openresty/lualib/resty
  cd -
  rm -rf ./lua-var-nginx-module-$ver
)

function install-offline() (

  tree $openresty/lualib
  mkdir -p $openresty/site/lualib/resty/
  tree $openresty/site/lualib/resty/

  # wget "https://opm.openresty.org/api/pkg/fetch?account=thibaultcha&name=lua-resty-mlcache&op=eq&version=2.5.0" -O ./lua-resty-mlcache-2.5.0.opm.tar
  # md5sum   ./lua-resty-mlcache-2.5.0.opm.tar ea5d142ffef2bea41ea408ef9aa94033
  wget http://prod-minio.alauda.cn/acp/ci/alb/build/lua-resty-mlcache-$LUA_RESTY_MLCACHE_VERSION.opm.tar -O ./lua-resty-mlcache-$LUA_RESTY_MLCACHE_VERSION.opm.tar
  tar -x -f ./lua-resty-mlcache-$LUA_RESTY_MLCACHE_VERSION.opm.tar
  cp -r ./lua-resty-mlcache-$LUA_RESTY_MLCACHE_VERSION.opm/lib/resty/* $openresty/site/lualib/resty

  # wget "https://opm.openresty.org/api/pkg/fetch?account=xiangnanscu&name=lua-resty-cookie&op=eq&version=0.01" -O ./lua-resty-cookie-0.01.opm.tar
  # md5sum ./lua-resty-cookie-0.01.opm.tar cfd011d1eb1712b47abd9cdffb7bc90b
  wget http://prod-minio.alauda.cn/acp/ci/alb/build/lua-resty-cookie-$LUA_RESTY_COOKIE_VERSION.opm.tar -O ./lua-resty-cookie-$LUA_RESTY_COOKIE_VERSION.opm.tar
  tar -x -f ./lua-resty-cookie-$LUA_RESTY_COOKIE_VERSION.opm.tar
  cp -r ./lua-resty-cookie-$LUA_RESTY_COOKIE_VERSION.opm/lib/resty/* $openresty/site/lualib/resty

  install-lua-var-nginx-module "http://prod-minio.alauda.cn/acp/ci/alb/build/lua-var-nginx-module-v${LUA_VAR_NGINX_MODULE_VERSION}.tar.gz" ${LUA_VAR_NGINX_MODULE_VERSION}
  install-lua-resty-balancer "http://prod-minio.alauda.cn/acp/ci/alb/build/lua-resty-balancer-v${LUA_RESTY_BALANCER_VERSION}.tar.gz" ${LUA_RESTY_BALANCER_VERSION}

  tree $openresty/lualib
  tree $openresty/site/lualib/resty/
  return
)

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
  install-offline
fi
