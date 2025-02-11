#!/bin/bash
# shellcheck disable=SC2046,SC2086,SC1091,SC2005

export GOPROXY=https://goproxy.cn,https://build-nexus.alauda.cn/repository/golang,direct

function _current_file() {
  local cf="${BASH_SOURCE[0]}"
  if [ -z "$cf" ]; then
    cf="$1"
  fi
  echo $cf
}

function _alb_dir() (
  echo $(dirname $(cd $(dirname $(_current_file $1)) && pwd))
)

CUR_ALB_BASE=$(_alb_dir "$0")

source $CUR_ALB_BASE/scripts/alb-test-actions.sh
source $CUR_ALB_BASE/scripts/alb-lint-actions.sh
source $CUR_ALB_BASE/scripts/alb-build-actions.sh
source $CUR_ALB_BASE/scripts/alb-env-actions.sh
source $CUR_ALB_BASE/scripts/alb-codegen-actions.sh
source $CUR_ALB_BASE/scripts/alb-perf-actions.sh
source $CUR_ALB_BASE/template/actions/alb-nginx.sh
source $CUR_ALB_BASE/scripts/alb-github-actions.sh

function alb-dev-install() (
  # yq => yq (https://github.com/mikefarah/yq/) version > 4
  return
)

function alb-list-todo-all() (
  rg TODO
)

function alb-list-todo-cur() (
  git diff $(git merge-base master HEAD)..HEAD | grep -i "TODO"
)

function alb-replace-alb() (
  alb-static-build
  local alb_pod=$(kubectl get po -n cpaas-system --no-headers | grep $1 | awk '{print $1}')
  kubectl cp $PWD/bin/alb cpaas-system/$alb_pod:/alb/ctl/alb -c alb2
)
