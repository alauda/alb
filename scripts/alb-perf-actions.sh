#!/bin/bash

function _check_require() (
  # alb-nginx
  # stapxx
  # fix-lua
  # stackcollapse-stap.pl
  # kernel has debug info
  return
)

function alb-perf-all() (
  local base=./.flame/perf-all
  mkdir -p $base
  cd ./.flame/perf-all
  alb-perf "http_r1" 50 $base/http_r1
  alb-perf "https_r1" 50 $base/https_r1
  alb-perf "http_r500" 50 $base/http_r500
  alb-perf "https_r500" 50 $base/https_r500
)

# 使用test-nginx启动nginx，然后使用fortio进行压测，同时使用stapxx进行lua flamegraph采集
# host probe host
function alb-perf() (
  sudo ls
  ulimit -n unlimited

  local perf_case=${1-"http_r1"}
  local perf_time=${2-"50"} # 50s 已经足够了
  local base=${4-"$(_alb_perf_init_base $perf_case)"}
  cd $base

  echo "run alb-ngix case $perf_case $perf_time seconds"
  _run_alb_nginx $perf_case $perf_time &
  date
  sleep 3 # wait nginx start

  local master_pid=$(cat $CUR_ALB_BASE/template/servroot/logs/nginx.pid)
  local worker_id=$(ps --ppid $master_pid -o pid=)
  worker_id=$(echo $worker_id | xargs)
  echo "master pid is $master_pid"
  echo "worker pid is |$worker_id|"

  alb-flame-stap-lua $perf_case $worker_id "$((perf_time - 20))" &
  alb-flame-perf-ng $perf_case $worker_id 20 &
  sleep 3 # wait stap start

  # time to run fortio
  local qps=$(fortio-test-alb $perf_case $perf_time)
  # wait other process
  wait
  echo "==========> $qps"
)

# alb在容器中启动，然后使用fortio进行压测，同时使用stapxx进行lua flamegraph采集
# host probe container
function alb-perf-docker() (
  # notic the stap used in stap++ is wg-stap...
  local perf_case=${1-"docker_http_r1"}
  local perf_time=${2-"50"} # 50s 已经足够了
  local container_name=${3-$(docker ps | fzf | awk '{print $1}')}
  local base=${4-"$(_alb_perf_init_base $perf_case)"}
  cd $base

  echo "run perf $perf_case $perf_time seconds. base $base"

  local worker_id=$(_apd_find_ng_worker_pid_in_docker $container_name)
  local root=$(_alb_perf_find_container_root $container_name)
  echo "worker pid is |$worker_id| $root"

  alb-flame-stap-lua $perf_case $worker_id "$((perf_time - 20))" "$root" &
  alb-flame-perf-ng $perf_case $worker_id 23 &
  echo "---$LINENO---"
  sleep 3 # wait stap start
  set -x
  echo "---$LINENO---"
  # time to run fortio
  local qps=$(fortio-test-alb $perf_case $perf_time "127.0.0.1")
  # wait other process
  wait
  echo "==========> $qps"
)

function fortio-test-alb() (
  echo "here"
  local perf_case="$1"
  local perf_time="$2"
  local ip=${3-"127.0.0.1"}
  local fortio_duration=$((perf_time - 10))
  local port="80"
  local scheme="http"
  local flag=""
  if [[ "$perf_case" =~ .*"https".* ]]; then
    port="443"
    scheme="https"
    flag="--https-insecure"
  fi
  echo "fortio load for $fortio_duration seconds"
  fortio load $flag -a -labels "u:alb_$perf_case f:c8_q3000_n_u_t${fortio_duration}s_s1024.99" -logger-force-color -c 8 -qps 30000 -nocatchup -uniform -t "${fortio_duration}s" "$scheme://$ip:$port?size=1024:99" &>./fortio.log &
  wait
  local qps=$(cat ./fortio.log | grep Ended | grep qps= | awk '{print $7}')
  echo "$qps"
)

function _alb_perf_init_base() {
  local perf_case=$1
  local suffix=$(date +%s)
  if [[ -n "$DEV_MODE" ]]; then
    suffix="dev"
  fi
  local base=./.flame/$perf_case-$suffix
  mkdir -p $base
  echo "$base"
}

function _alb_perf_find_container_root() {
  local root=$(docker inspect $1 | jq '.[0].GraphDriver.Data.MergedDir')
  echo $root
}

function fortio-server() (
  echo "start fortio server"
  fortio server
  return
)

function alb-perf-watch-ng() (
  while true; do
    sleep 1s
    clear
    date
    ps -aux | grep nginx | grep -v tail | grep -v grep | grep -v stap
  done
)

function alb-perf-watch-ng-qps() (
  while true; do
    date
    sleep 1s
    local ng_pid=$(ps -aux | grep nginx | grep worker | awk '{print $2}')
    if [[ -z "$ng_pid" ]]; then
      echo "ngx pid not find"
      continue
    fi
    echo "ngx pid is $ng_pid"
    cd $STAPXX_BASE
    pwd
    export PATH=$PATH:$STAPXX_BASE
    sudo ./samples/ngx-rps.sxx -x $ng_pid
  done
)

function _ng_stap_lua_gen_svg() (
  local bt=$1
  local svg=$2
  local root=${3-""}
  cat $bt | tr -cd '\11\12\15\40-\176' >$bt.clean # backtrace 中可能有乱码。。一般是解析luajit的trace的name的时候名字拿不到。。。
  sudo bash -c "export FIX_LUA_ROOT=\"$root\"; fix-lua-bt $bt.clean > $bt.fixlua"
  stackcollapse-stap.pl $bt.fixlua >$bt.stackcollapse
  flamegraph.pl --encoding="utf-8" --title="Lua-land on-CPU flamegraph" $bt.stackcollapse >$svg
)

function _run_alb_nginx() (
  export KEEP_TWEAK="true"
  export PERF_CASE=$1
  export PERF_TIME=$2
  test-nginx-in-ci $CUR_ALB_BASE/template/t/perf/e2e/fortio.perf
)

function alb-flame-stap-lua() (
  local key=$1
  mkdir -p ./.fg/$key
  local pid=$2
  if [[ -z "$pid" ]]; then
    pid=$(cat ./template/servroot/logs/nginx.pid)
    echo "$pid"
    ps -aux | grep $pid
    sleep 3
  fi
  local time=${3-"120"}
  local root=${4-""}
  echo "start perf stap lua"
  echo "capture lua stack for $pid for $time seconds root $root"
  local flag=""
  if [[ -n "$root" ]]; then
    flag="--sysroot $root"
  fi
  eval "sudo $STAPXX_BASE/stap++ $flag $STAPXX_BASE/samples/lj-lua-stacks.sxx --skip-badvars -x $pid --arg time=$time >$key.bt"
  echo "stop capture lua stack for $pid"
  _ng_stap_lua_gen_svg ./.fg/$key/$key.bt ./.fg/$key/$key.svg $root
  if [[ -n "$OPEN_SVG" ]]; then
    firefox ./.fg/$key/$key.svg
  fi
)

function alb-flame-perf-ng() (
  local key=$1
  mkdir -p ./.fg/$key
  local pid=$2
  if [[ -z "$pid" ]]; then
    pid=$(cat ./template/servroot/logs/nginx.pid)
    echo "$pid"
    ps -aux | grep $pid
    sleep 3
  fi
  local time=${2-"20"}
  echo "capture nginx stack for $pid for $time seconds"
  sudo perf record -a -g -p $pid --call-graph dwarf -- sleep $time
  sudo perf script | stackcollapse-perf.pl | flamegraph.pl >./.fg/$key/nginx.svg
  if [[ -n "$OPEN_SVG" ]]; then
    firefox ./.fg/$key/nginx.svg
  fi
)

function apd-run-docker() {
  local alb_image="$1"
  if [ ! -d $PWD/tweak ]; then
    tweak_gen ./tweak
  fi
  docker run --name alb-ng --network host -it --env-file $PWD/store/env -v $PWD/tweak:/alb/tweak -v $PWD/store/nginx.conf:/etc/alb2/nginx/nginx.conf -v $PWD/store/policy.json:/etc/alb2/nginx/policy.new --privileged -u root:root $alb_image /alb/nginx/run-nginx.sh
}

function _apd_find_ng_worker_pid_in_docker() (
  function _docker-ps-via-id() (
    local id=$1
    local pid_of_container=$(docker inspect $id | jq '.[0].State.Pid')
    local pidns=$(sudo sh -c "ls -l /proc/$pid_of_container/ns/pid|grep -o '\[.*\]'|tr -d '[]'")
    while read pid; do
      local pid_info_in_host=$(ps -o pid,uid,cmd -p $pid --no-headers)
      local pid_in_ns=$(cat /proc/$pid/status | grep NSpid | awk '{print $3}')
      echo $pid_in_ns $pid_info_in_host
    done < <(sudo sh -c "ls -l /proc/*/ns/pid|grep $pidns | awk '{print \$9}'| awk -F'/' '{print \$3}'")
  )

  local container_name=$1
  local pid=$(_docker-ps-via-id $container_name | grep 'nginx: worker process' | awk '{print $2}' | head -n1)
  echo $pid
)

function alb-perf-go-policy-gen() (
  export RULE_PERF="true"
  ginkgo -focus "should ok when has 5k rule" -v ./test/e2e
  go tool pprof -raw ./test/e2e/rule-perf-cpu >cpu.raw
  stackcollapse-go.pl ./cpu.raw >cpu.folded
  flamegraph.pl ./cpu.folded >./cpu.svg
)
