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

tree $openresty/site/
opm install thibaultcha/lua-resty-mlcache=$LUA_RESTY_MLCACHE_VERSION
tree $openresty/site/
opm install xiangnanscu/lua-resty-cookie=$LUA_RESTY_COOKIE_VERSION
tree $openresty/site/

# lua-resty-balancer
tree $openresty/lualib/resty
(
  curl -fSL https://github.com/openresty/lua-resty-balancer/archive/v${LUA_RESTY_BALANCER_VERSION}.tar.gz -o lua-resty-balancer-v${LUA_RESTY_BALANCER_VERSION}.tar.gz
  tar xzf lua-resty-balancer-v${LUA_RESTY_BALANCER_VERSION}.tar.gz && rm -rf lua-resty-balancer-v${LUA_RESTY_BALANCER_VERSION}.tar.gz
  cd lua-resty-balancer-${LUA_RESTY_BALANCER_VERSION}
  export LUA_LIB_DIR=$openresty/lualib
  make && make install
  cd -
  rm -rf ./lua-resty-balancer-${LUA_RESTY_BALANCER_VERSION}
)

tree $openresty/lualib/resty
# lua-var-nginx-module
(
  curl -fSL https://github.com/api7/lua-var-nginx-module/archive/v${LUA_VAR_NGINX_MODULE_VERSION}.tar.gz -o lua-var-nginx-module-v${LUA_VAR_NGINX_MODULE_VERSION}.tar.gz
  tar xzf lua-var-nginx-module-v${LUA_VAR_NGINX_MODULE_VERSION}.tar.gz
  rm -rf lua-var-nginx-module-v${LUA_VAR_NGINX_MODULE_VERSION}.tar.gz
  cd lua-var-nginx-module-${LUA_VAR_NGINX_MODULE_VERSION}
  ls lib/resty/*
  cp -r lib/resty/* $openresty/lualib/resty
  cd -
  rm -rf ./lua-var-nginx-module-${LUA_VAR_NGINX_MODULE_VERSION}
)
tree $openresty/lualib/resty
