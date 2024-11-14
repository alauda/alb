#!/bin/bash

# set -e

LUA_VAR_NGINX_MODULE_VERSION="0.5.2"
LUA_RESTY_BALANCER_VERSION="0.04"
LUA_RESTY_MLCACHE_VERSION="2.5.0"
LUA_RESTY_COOKIE_VERSION="0.01"
LUA_PROTOBUF="0.5.2"    # used by opentelemetry-lua
LUA_RESTY_HTTP="0.16.1" # used by  opentelemetry-lua and our nginx test # 0.17.2 enable mtls in default and we don's need it yet.
OPENTELEMETRY_LUA="0.2.6"

openresty=$1
if [[ -n "$OPENRESTY_BUILD_TRARGRT_DIR" ]]; then
  openresty=$OPENRESTY_BUILD_TRARGRT_DIR
fi

if [[ -z "$openresty" && -d "/usr/local/openresty" ]]; then
  openresty="/usr/local/openresty"
  echo "use default $openresty "
fi

if [[ -z "$openresty" && -d "/opt/openresty" ]]; then
  openresty="/opt/openresty"
  echo "use default $openresty "
fi

if [ -z "$openresty" ]; then
  echo "Usage: $0 /path/to/openresty"
  exit 1
fi

function alb-ng-install-test-deps() (
  luarocks --lua-version 5.1 --tree $openresty/luajit install luacov
  luarocks --lua-version 5.1 --tree $openresty/luajit install cluacov
  luarocks --lua-version 5.1 install luacov
  luarocks --lua-version 5.1 install luacov-console
  luarocks --lua-version 5.1 install luacov-html
)

function alb-ng-install-deps() (
  env
  mkdir -p $openresty/site/lualib/resty/
  tree $openresty/luajit
  tree $openresty/site/
  install-lua-resty-mlcache
  install-lua-var-nginx-module
  install-lua-resty-balancer
  install-lua-resty-cookie
  install-lua-protobuf
  install-opentelemetry-lua
  install-lua-resty-http
  alb-ng-install-waf
  tree $openresty/lualib/resty
  tree /etc/nginx
)

function alb-ng-install-waf() (
  export OWASP_MODSECURITY_CRS_VERSION=4.4.0
  # https://github.com/kubernetes/ingress-nginx/blob/c9743ae58599ad43236a7b28aea0f6bd8cd19c4a/images/nginx-1.25/rootfs/build.sh#L363
  mkdir -p /etc/nginx/modsecurity
  local recommended_conf_url_online="https://raw.githubusercontent.com/owasp-modsecurity/ModSecurity/v3/master/modsecurity.conf-recommended"
  local recommended_conf_url_offline="http://prod-minio.alauda.cn/acp/ci/alb/build/modsecurity.conf-recommended"
  # f82971c3764647d751372fa423bd56fe  ./modsecurity.conf-recommended
  local recommended_url=$(_alb_lua_switch "$recommended_conf_url_online" "$recommended_conf_url_offline")

  local coreruleset_url_online="https://codeload.github.com/coreruleset/coreruleset/zip/refs/tags/v$OWASP_MODSECURITY_CRS_VERSION"
  local coreruleset_url_offline="http://prod-minio.alauda.cn/acp/ci/alb/build/coreruleset-4.4.0.zip"
  local coreruleset_url=$(_alb_lua_switch "$coreruleset_url_online" "$coreruleset_url_offline")

  local unicode_mapping_url_online="https://raw.githubusercontent.com/owasp-modsecurity/ModSecurity/v3/master/unicode.mapping"
  local unicode_mapping_url_offline="http://prod-minio.alauda.cn/acp/ci/alb/build/unicode.mapping"
  local unicode_mapping_url=$(_alb_lua_switch "$unicode_mapping_url_online" "$unicode_mapping_url_offline")

  rm -rf /etc/nginx/modsecurity || true
  rm -rf /etc/nginx/owasp-modsecurity-crs || true
  mkdir -p /etc/nginx/modsecurity/
  mkdir -p /var/log/audit/
  wget $recommended_url -O /etc/nginx/modsecurity/modsecurity.conf
  wget $unicode_mapping_url -O /etc/nginx/modsecurity/unicode.mapping

  # Replace serial logging with concurrent
  sed -i 's|SecAuditLogType Serial|SecAuditLogType Concurrent|g' /etc/nginx/modsecurity/modsecurity.conf

  # Concurrent logging implies the log is stored in several files
  echo "SecAuditLogStorageDir /var/log/audit/" >>/etc/nginx/modsecurity/modsecurity.conf

  # Download owasp modsecurity crs
  cd /etc/nginx/
  set +x
  wget $coreruleset_url -O ./coreruleset.zip
  unzip ./coreruleset.zip
  mv ./coreruleset-$OWASP_MODSECURITY_CRS_VERSION ./owasp-modsecurity-crs
  rm ./coreruleset.zip
  cd ./owasp-modsecurity-crs

  mv crs-setup.conf.example crs-setup.conf
  mv rules/REQUEST-900-EXCLUSION-RULES-BEFORE-CRS.conf.example rules/REQUEST-900-EXCLUSION-RULES-BEFORE-CRS.conf
  mv rules/RESPONSE-999-EXCLUSION-RULES-AFTER-CRS.conf.example rules/RESPONSE-999-EXCLUSION-RULES-AFTER-CRS.conf
  cd ..
  set +x

  # OWASP CRS v4 rules
  echo "
Include /etc/nginx/owasp-modsecurity-crs/crs-setup.conf
Include /etc/nginx/owasp-modsecurity-crs/rules/REQUEST-900-EXCLUSION-RULES-BEFORE-CRS.conf
Include /etc/nginx/owasp-modsecurity-crs/rules/REQUEST-901-INITIALIZATION.conf
Include /etc/nginx/owasp-modsecurity-crs/rules/REQUEST-905-COMMON-EXCEPTIONS.conf
Include /etc/nginx/owasp-modsecurity-crs/rules/REQUEST-911-METHOD-ENFORCEMENT.conf
Include /etc/nginx/owasp-modsecurity-crs/rules/REQUEST-913-SCANNER-DETECTION.conf
Include /etc/nginx/owasp-modsecurity-crs/rules/REQUEST-920-PROTOCOL-ENFORCEMENT.conf
Include /etc/nginx/owasp-modsecurity-crs/rules/REQUEST-921-PROTOCOL-ATTACK.conf
Include /etc/nginx/owasp-modsecurity-crs/rules/REQUEST-922-MULTIPART-ATTACK.conf
Include /etc/nginx/owasp-modsecurity-crs/rules/REQUEST-930-APPLICATION-ATTACK-LFI.conf
Include /etc/nginx/owasp-modsecurity-crs/rules/REQUEST-931-APPLICATION-ATTACK-RFI.conf
Include /etc/nginx/owasp-modsecurity-crs/rules/REQUEST-932-APPLICATION-ATTACK-RCE.conf
Include /etc/nginx/owasp-modsecurity-crs/rules/REQUEST-933-APPLICATION-ATTACK-PHP.conf
Include /etc/nginx/owasp-modsecurity-crs/rules/REQUEST-934-APPLICATION-ATTACK-GENERIC.conf
Include /etc/nginx/owasp-modsecurity-crs/rules/REQUEST-941-APPLICATION-ATTACK-XSS.conf
Include /etc/nginx/owasp-modsecurity-crs/rules/REQUEST-942-APPLICATION-ATTACK-SQLI.conf
Include /etc/nginx/owasp-modsecurity-crs/rules/REQUEST-943-APPLICATION-ATTACK-SESSION-FIXATION.conf
Include /etc/nginx/owasp-modsecurity-crs/rules/REQUEST-944-APPLICATION-ATTACK-JAVA.conf
Include /etc/nginx/owasp-modsecurity-crs/rules/REQUEST-949-BLOCKING-EVALUATION.conf
Include /etc/nginx/owasp-modsecurity-crs/rules/RESPONSE-950-DATA-LEAKAGES.conf
Include /etc/nginx/owasp-modsecurity-crs/rules/RESPONSE-951-DATA-LEAKAGES-SQL.conf
Include /etc/nginx/owasp-modsecurity-crs/rules/RESPONSE-952-DATA-LEAKAGES-JAVA.conf
Include /etc/nginx/owasp-modsecurity-crs/rules/RESPONSE-953-DATA-LEAKAGES-PHP.conf
Include /etc/nginx/owasp-modsecurity-crs/rules/RESPONSE-954-DATA-LEAKAGES-IIS.conf
Include /etc/nginx/owasp-modsecurity-crs/rules/RESPONSE-955-WEB-SHELLS.conf
Include /etc/nginx/owasp-modsecurity-crs/rules/RESPONSE-959-BLOCKING-EVALUATION.conf
Include /etc/nginx/owasp-modsecurity-crs/rules/RESPONSE-980-CORRELATION.conf
Include /etc/nginx/owasp-modsecurity-crs/rules/RESPONSE-999-EXCLUSION-RULES-AFTER-CRS.conf
" >/etc/nginx/owasp-modsecurity-crs/nginx-modsecurity.conf
  return
)

function _alb_lua_switch() (
  local online=$1
  local offline=$2
  if [[ "$(_alb_am_i_online)" == "true" ]]; then
    echo "$online"
    return
  fi
  echo "$offline"
)

function _alb_am_i_online() (
  if [[ "$ALB_ONLINE" == "true" ]]; then
    echo "true"
    return
  fi
  echo "false"
)

function install-lua-resty-http() (
  # md5sum ./lua-resty-http-0.17.2.zip
  # 043db2984a5a1dc3d66e605568ed8adb  ./lua-resty-http-0.17.2.zip
  local ver="$LUA_RESTY_HTTP"

  local online="https://codeload.github.com/ledgetech/lua-resty-http/zip/refs/tags/v$ver"
  local offline="http://prod-minio.alauda.cn/acp/ci/alb/build/lua-resty-http-$ver.zip"
  local name="lua-resty-http"
  local url=$(_alb_lua_switch $online $offline)
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
  local url=$(_alb_lua_switch $online $offline)
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
  # md5sum   ./lua-resty-mlcache-2.5.0.opm.tar ea5d142ffef2bea41ea408ef9aa94033
  local online="https://opm.openresty.org/api/pkg/fetch?account=thibaultcha&name=lua-resty-mlcache&op=eq&version=$LUA_RESTY_MLCACHE_VERSION"
  local offline="http://prod-minio.alauda.cn/acp/ci/alb/build/lua-resty-mlcache-$LUA_RESTY_MLCACHE_VERSION.opm.tar"
  local url=$(_alb_lua_switch $online $offline)
  wget $url -O ./lua-resty-mlcache-$LUA_RESTY_MLCACHE_VERSION.opm.tar
  tar -x -f ./lua-resty-mlcache-$LUA_RESTY_MLCACHE_VERSION.opm.tar
  cp -r ./lua-resty-mlcache-$LUA_RESTY_MLCACHE_VERSION.opm/lib/resty/* $openresty/site/lualib/resty
  rm -rf ./lua-resty-mlcache-$LUA_RESTY_MLCACHE_VERSION*
)

function install-lua-resty-cookie() (
  # md5sum ./lua-resty-cookie-0.01.opm.tar cfd011d1eb1712b47abd9cdffb7bc90b
  local online="https://opm.openresty.org/api/pkg/fetch?account=xiangnanscu&name=lua-resty-cookie&op=eq&version=$LUA_RESTY_COOKIE_VERSION"
  local offline="http://prod-minio.alauda.cn/acp/ci/alb/build/lua-resty-cookie-$LUA_RESTY_COOKIE_VERSION.opm.tar"
  local url=$(_alb_lua_switch $online $offline)
  wget $url -O ./lua-resty-cookie-$LUA_RESTY_COOKIE_VERSION.opm.tar
  tar -x -f ./lua-resty-cookie-$LUA_RESTY_COOKIE_VERSION.opm.tar
  cp -r ./lua-resty-cookie-$LUA_RESTY_COOKIE_VERSION.opm/lib/resty/* $openresty/site/lualib/resty
  rm -rf ./lua-resty-cookie-$LUA_RESTY_COOKIE_VERSION*
)

function install-lua-resty-balancer() (
  local online="https://github.com/openresty/lua-resty-balancer/archive/v${LUA_RESTY_BALANCER_VERSION}.tar.gz"
  local offline="http://prod-minio.alauda.cn/acp/ci/alb/build/lua-resty-balancer-v${LUA_RESTY_BALANCER_VERSION}.tar.gz"
  local url=$(_alb_lua_switch $online $offline)
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
  local online="https://github.com/api7/lua-var-nginx-module/archive/v${LUA_VAR_NGINX_MODULE_VERSION}.tar.gz"
  local offline="http://prod-minio.alauda.cn/acp/ci/alb/build/lua-var-nginx-module-v${LUA_VAR_NGINX_MODULE_VERSION}.tar.gz"
  local url=$(_alb_lua_switch $online $offline)
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
  local url=$(_alb_lua_switch $online $offline)
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

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
  alb-ng-install-deps
fi
