#!/bin/bash
source ./template/actions/dev.actions.sh
source ./scripts/alb-lint-actions.sh

if [[ -n "$CUR_ALB_BASE" ]]; then
  export ALB=$CUR_ALB_BASE
fi

function alb-install-nginx-test-dependency() {
  apk update && apk add luarocks luacheck lua-dev lua perl-app-cpanminus wget curl make build-base perl-dev git neovim bash yq jq tree fd openssl
  mkdir /tmp && export TMP=/tmp # luarocks need this
  cp /usr/bin/luarocks-5.1 /usr/bin/luarocks
  cpanm --mirror-only --mirror https://mirrors.tuna.tsinghua.edu.cn/CPAN/ -v --notest Test::Nginx IPC::Run YAML::PP
  luarocks
  (
    source ./template/actions/alb-nginx-install-deps.sh
    alb-ng-install-test-deps
  )
}

function alb-install-nginx-test-dependency-ubuntu() {
  sudo cpanm --mirror-only --mirror https://mirrors.tuna.tsinghua.edu.cn/CPAN/ -v --notest Test::Nginx IPC::Run YAML::PP
}

function alb-install-nginx-test-dependency-arch() {
  yay -S luarocks luacheck lua51 perl-app-cpanminus wget curl make base-devel cpanminus perl git neovim bash yq jq tree fd openssl
  # export PATH=/opt/openresty/bin:$PATH
  # export PATH=/opt/openresty/nginx/sbin:$PATH
  # init the openresty
  sudo cpanm --mirror-only --mirror https://mirrors.tuna.tsinghua.edu.cn/CPAN/ -v --notest Test::Nginx IPC::Run YAML::PP
}

function tweak_gen_install() {
  go build -v -v -o ./bin/tools/tweak_gen alauda.io/alb2/cmd/utils/tweak_gen
  md5sum ./bin/tools/tweak_gen
  sudo cp ./bin/tools/tweak_gen /usr/local/bin
}

function ngx_gen_install() {
  go build -v -v -o ./bin/tools/ngx_gen alauda.io/alb2/cmd/utils/ngx_gen
  md5sum ./bin/tools/ngx_gen
  sudo cp ./bin/tools/ngx_gen /usr/local/bin
}

if [[ -n "$CUR_ALB_BASE" ]]; then
  export ALB=$CUR_ALB_BASE
fi

function alb-test-all-in-ci-nginx() {
  # base image build-harbor.alauda.cn/3rdparty/alb-nginx:v3.9-57-gb40a7de
  set -ex # exit on err
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
  export LUACOV=true
  test-nginx-in-ci
  local end_test=$(date +"%Y %m %e %T.%6N")
  echo "start " $start
  echo "install " $end_install
  echo "check" $end_check
  echo "test" $end_test
  pwd
  luacov-console $PWD/template/nginx/lua/
  luacov-console $PWD/template/nginx/lua/ -s
  luacov-console $PWD/template/nginx/lua/ -s >./luacov.summary
}

function test-nginx-local() {
  #   test-nginx-in-ci $PWD/template/t/ping.t # run special test
  #   test-nginx-in-ci                        # run all test
  # make sure tweak_gen is in path
  # clean up if you use run-like-ci-nginx.sh before
  sudo rm -rf ./template/logs
  sudo rm -rf ./template/cert
  sudo rm -rf ./template/servroot
  sudo rm -rf ./template/share/
  sudo rm -rf ./template/policy.new

  # sudo setcap CAP_NET_BIND_SERVICE=+eip `which nginx`
  local t="ping"
  if [ -n "$1" ]; then
    t="$1"
  fi
  test-nginx-in-ci $PWD/template/t/$t.t
}

function test-nginx-in-ci() (
  alb-nginx-test $1
)

function alb-nginx-test() (
  set -e
  set -x
  echo "alb-nginx-test" alb is $ALB
  local t1=$(date)
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
  export METRICS_AUTH="false"
  export MY_POD_NAME="mock-alb-pod"
  export NAME="test-alb"
  export ALB_NS="cpaas-system"
  export ALB_VER="3.18.0"
  export HOSTNAME="the-host"

  mkdir -p $TEST_BASE/cert

  if [[ "$KEEP_TWEAK" != "true" ]]; then
    rm -rf $TEST_BASE/tweak
  fi
  if [[ ! -d $TEST_BASE/tweak ]]; then
    configmap_to_file $TEST_BASE/tweak
  fi
  mkdir -p $TEST_BASE/share
  openssl dhparam -dsaparam -out $TEST_BASE/share/dhparam.pem 2048
  local filter=""
  if [ -z "$1" ]; then
    filter="$TEST_BASE/t"
  else
    filter=$1
  fi
  ls $TEST_BASE
  unset http_proxy
  unset https_proxy
  local t2=$(date)
  prove -I $TEST_BASE/ -r $filter
  local t3=$(date)
  echo $t1
  echo $t2
  echo $t3
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

function alb-nginx-watch-log() (
  echo "watch log"
  tail -F ./template/servroot/logs/error.log | python -u -c '
import sys
for line in sys.stdin:
    if "keepalive connection" in line:
        continue
    if line.startswith("20"):
        spos = line.find("[lua]") + 5
        epos = line.find("client: 127.0.0.1,")
        time=line.split()[1].strip()
        print("|->  "+time+" "+line[spos:epos].strip()+"  <-|")
        continue
    if line.strip().startswith(","): 
        continue
    print("e|"+line)
'
)

# we donot want to same name lua file. some tool, like jit dump could only show file name.
function alb-nginx-list-lib-lua() (
  local openresty=$(echo $(realpath $(which nginx)) | sed 's|nginx/sbin/nginx||g')
  local lib=$(fd . $openresty | grep '\.lua')
  echo "$lib"
)

function alb-nginx-list-lib-lua-name() (
  alb-nginx-list-lib-lua | xargs -I{} basename {} | sort | uniq
)

function alb-nginx-list-our-lua() (
  fd . ./template/nginx/lua | grep '\.lua'
)

function alb-nginx-list-our-lua-name() (
  alb-nginx-list-our-lua | xargs -I{} basename {} | sort | uniq
)

function alb-nginx-check-filename() (
  local lib=$(alb-nginx-list-lib-lua-name)
  local our=$(alb-nginx-list-our-lua-name)
  local find="false"
  while read -r line; do
    local name=$(echo $line | awk '{print $2}')
    echo "$name in lib" $(alb-nginx-list-lib-lua | grep $name)
    echo "$name in our" $(alb-nginx-list-our-lua | grep $name)
    find="true"
  done < <(echo "$lib\n$our" | sort | uniq -c | grep -v "1 ")
  if [[ "$find" == "true" ]]; then
    echo "find duplicate"
    exit 1
  fi
)

function alb-nginx-show-our-nyi() (
  local lib=$(alb-nginx-list-lib-lua-name)
  local nyi_raw=$(cat ./template/.*.nyi | grep NYI)
  local our=$(fd . ./template/nginx/lua | grep '\.lua' | xargs -I{} basename {} | sort | uniq)
  local nyi=$(echo "$nyi_raw" | rg '\s([^\s]*).lua' -o -r '$1' | sort | uniq)
  while read -r lua; do
    lua="$lua.lua"
    local inour=$(echo "$our" | grep $lua)
    local inlib=$(echo "$lib" | grep $lua)
    if [[ -n "$inour" ]]; then
      echo "- $lua | in our $inour | $(echo ""$nyi_raw"" | grep $lua) | \n\n "
    fi
    # if [[ -n "$inlib" ]]; then
    #    echo "- $lua | in lib $inlib |"
    # fi
  done < <(echo "$nyi" | grep -v 'prometheus')
  return
)

function alb-nginx-build-tylua() (
  cd ./cmd/utils/tylua
  rm ./bin/tylua || true
  mkdir -p ./bin
  go build -v -o ./bin/tylua
  md5sum ./bin/tylua
)

function alb-nginx-tylua() (
  local coverpkg_list=$(go list ./... | grep -v e2e | grep -v test | grep -v "/pkg/client" | grep -v migrate | sort | uniq)
  local coverpkg=$(echo "$coverpkg_list" | tr "\n" ",")
  ./cmd/utils/tylua/bin/tylua $coverpkg NgxPolicy ./template/nginx/lua/types/ngxpolicy.types.lua

  return
)
