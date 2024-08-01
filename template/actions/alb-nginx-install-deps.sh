#!/bin/bash

set -x
set -e

LUA_VAR_NGINX_MODULE_VERSION="0.5.2"
LUA_RESTY_BALANCER_VERSION="0.04"
LUA_RESTY_MLCACHE_VERSION="2.5.0"
LUA_RESTY_COOKIE_VERSION="0.01"
LUA_PROTOBUF="0.5.2"    # used by opentelemetry-lua
LUA_RESTY_HTTP="0.16.1" # used by  opentelemetry-lua and our nginx test # 0.17.2 enable mtls in default and we don's need it yet.
OPENTELEMETRY_LUA="0.2.6"

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

function install-offline() (
  mkdir -p $openresty/site/lualib/resty/
  tree $openresty/luajit

  install-lua-var-nginx-module
  install-lua-resty-mlcache
  install-lua-resty-cookie
  install-lua-resty-balancer
  install-lua-protobuf
  install-opentelemetry-lua
  install-lua-resty-http

  tree $openresty/luajit
  return
)

function install-lua-resty-http() (
  # md5sum ./lua-resty-http-0.17.2.zip
  # 043db2984a5a1dc3d66e605568ed8adb  ./lua-resty-http-0.17.2.zip
  local ver="$LUA_RESTY_HTTP"

  local online="https://codeload.github.com/ledgetech/lua-resty-http/zip/refs/tags/v$ver"
  local offline="http://prod-minio.alauda.cn/acp/ci/alb/build/lua-resty-http-$ver.zip"
  local name="lua-resty-http"
  local url=$offline
  if [[ "$1" == "online" ]]; then
    url=$online
  fi
  rm -rf ./.$name || true
  mkdir -p ./.$name
  cd ./.$name
  wget $url -O $name-$ver.zip
  unzip $name-$ver.zip
  cd ./$name-$ver
  ls
  luarocks --lua-version 5.1 --tree $openresty/luajit make --deps-mode none ./lua-resty-http-$ver-0.rockspec
  cd ../../
  rm -rf ./.$name
  return
)
function install-opentelemetry-lua() (
  # md5sum ./opentelemetry-lua-0.2.6.zip
  # 77f4488e669c80d53c3d9977f35017ed  ./opentelemetry-lua-0.2.6.zip
  local ver="$OPENTELEMETRY_LUA"
  local online="https://codeload.github.com/yangxikun/opentelemetry-lua/zip/refs/tags/v$ver"
  local offline="http://prod-minio.alauda.cn/acp/ci/alb/build/opentelemetry-lua-$ver.zip"
  local name="opentelemetry-lua"
  local url=$offline
  if [[ "$1" == "online" ]]; then
    url=$online
  fi
  rm -rf ./.$name || true
  mkdir -p ./.$name
  cd ./.$name
  wget $url -O $name-$ver.zip
  unzip $name-$ver.zip
  cd ./opentelemetry-lua-$ver
  ls
  local v=$(echo "$ver" | sed 's/\./-/2')
  luarocks --lua-version 5.1 --tree $openresty/luajit make --deps-mode none ./rockspec/opentelemetry-lua-$v.rockspec
  cd ../../
  rm -rf ./.$name
  return
)

function install-lua-resty-mlcache() (
  # wget "https://opm.openresty.org/api/pkg/fetch?account=thibaultcha&name=lua-resty-mlcache&op=eq&version=2.5.0" -O ./lua-resty-mlcache-2.5.0.opm.tar
  # md5sum   ./lua-resty-mlcache-2.5.0.opm.tar ea5d142ffef2bea41ea408ef9aa94033
  wget http://prod-minio.alauda.cn/acp/ci/alb/build/lua-resty-mlcache-$LUA_RESTY_MLCACHE_VERSION.opm.tar -O ./lua-resty-mlcache-$LUA_RESTY_MLCACHE_VERSION.opm.tar
  tar -x -f ./lua-resty-mlcache-$LUA_RESTY_MLCACHE_VERSION.opm.tar
  cp -r ./lua-resty-mlcache-$LUA_RESTY_MLCACHE_VERSION.opm/lib/resty/* $openresty/site/lualib/resty
  rm -rf ./lua-resty-mlcache-$LUA_RESTY_MLCACHE_VERSION*
)

function install-lua-resty-cookie() (
  # wget "https://opm.openresty.org/api/pkg/fetch?account=xiangnanscu&name=lua-resty-cookie&op=eq&version=0.01" -O ./lua-resty-cookie-0.01.opm.tar
  # md5sum ./lua-resty-cookie-0.01.opm.tar cfd011d1eb1712b47abd9cdffb7bc90b
  wget http://prod-minio.alauda.cn/acp/ci/alb/build/lua-resty-cookie-$LUA_RESTY_COOKIE_VERSION.opm.tar -O ./lua-resty-cookie-$LUA_RESTY_COOKIE_VERSION.opm.tar
  tar -x -f ./lua-resty-cookie-$LUA_RESTY_COOKIE_VERSION.opm.tar
  cp -r ./lua-resty-cookie-$LUA_RESTY_COOKIE_VERSION.opm/lib/resty/* $openresty/site/lualib/resty
  rm -rf ./lua-resty-cookie-$LUA_RESTY_COOKIE_VERSION*
)

function install-lua-resty-balancer() (
  local url="http://prod-minio.alauda.cn/acp/ci/alb/build/lua-resty-balancer-v${LUA_RESTY_BALANCER_VERSION}.tar.gz"
  local ver="${LUA_RESTY_BALANCER_VERSION}"
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
  local url="http://prod-minio.alauda.cn/acp/ci/alb/build/lua-var-nginx-module-v${LUA_VAR_NGINX_MODULE_VERSION}.tar.gz"
  local ver="${LUA_VAR_NGINX_MODULE_VERSION}"
  wget $url -O lua-var-nginx-module-v$ver.tar.gz
  tar xzf lua-var-nginx-module-v$ver.tar.gz
  rm -rf lua-var-nginx-module-v$ver.tar.gz
  cd lua-var-nginx-module-$ver
  ls lib/resty/*
  cp -r lib/resty/* $openresty/lualib/resty
  cd -
  rm -rf ./lua-var-nginx-module-$ver
)

function install-lua-protobuf() (
  local offline="http://prod-minio.alauda.cn/acp/ci/alb/build/lua-protobuf-$LUA_PROTOBUF.zip"
  local online="https://codeload.github.com/starwing/lua-protobuf/zip/refs/tags/$LUA_PROTOBUF"
  local name="lua-protobuf"
  local url=$offline
  if [[ "$1" == "online" ]]; then
    url=$online
  fi
  rm -rf ./$name || true
  mkdir -p ./lua-protobuf
  cd ./lua-protobuf
  wget $url -O lua-protobuf-$LUA_PROTOBUF.zip
  unzip lua-protobuf-$LUA_PROTOBUF.zip
  cd ./lua-protobuf-$LUA_PROTOBUF
  luarocks --lua-version 5.1 --tree $openresty/luajit make --deps-mode none ./rockspecs/lua-protobuf-scm-1.rockspec
  cd ../../
  rm -rf ./lua-protobuf
  return
)

function install-online() (
  tree $openresty/site/
  opm install thibaultcha/lua-resty-mlcache=$LUA_RESTY_MLCACHE_VERSION
  tree $openresty/site/
  opm install xiangnanscu/lua-resty-cookie=$LUA_RESTY_COOKIE_VERSION
  tree $openresty/site/

  install-lua-var-nginx-module "https://github.com/api7/lua-var-nginx-module/archive/v${LUA_VAR_NGINX_MODULE_VERSION}.tar.gz" ${LUA_VAR_NGINX_MODULE_VERSION}
  install-lua-resty-balancer "https://github.com/openresty/lua-resty-balancer/archive/v${LUA_RESTY_BALANCER_VERSION}.tar.gz" ${LUA_RESTY_BALANCER_VERSION}
  install-lua-protobuf "online"
  install-opentelemetry-lua "online"
  install-lua-resty-http "online"
  tree $openresty/lualib/resty
)

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
  install-offline
fi
