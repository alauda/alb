#!/bin/bash

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
