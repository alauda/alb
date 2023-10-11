#!/bin/bash

# 3.12.0 3.12.1 3.12.2 上
# 1. 资源为空时，升级到operator时，会设置默认的资源，但是nginx给的默认资源太小了。所有这里检查下如果没有显示的设置资源，就让他设置
# 2. 资源的cpu的limit的格式不能为x000m 这种格式。
# 上述问题 3.14.0 及其之后已经修复了

# Check Alb Resource Log Info
function _car_log_info() {
  log::info "check-alb-resource $@"
  return
}

# Check Alb Resource Log Error
function _car_log_err() {
  log::err "check-alb-resource $@"
  return
}

function _car_check_defualt_alb() (
  local cluster="$1"
  _car_log_info "====检查 $cluster 集群的默认alb===="

  local cpulimit=$(kubectl-get-alb-apprelease-values-with-cluster global resources.limits.cpu)
  local name=$(kubectl-get-alb-apprelease-values-with-cluster global loadbalancerName)
  if [[ -z "$cpulimit" ]]; then
    _car_log_err "====$cluster 集群默认alb $name 的apprelease中没有设置cpulimit 请检查===="
  fi
  if [[ "$cpulimit" == *"000m"* ]]; then
    _car_log_err "====$cluster 集群默认alb $name 的apprelease中cpulimit格式不正确 请检查===="
  fi
  return
)

function _car_check_user_alb() (
  local cluster="$1"
  _car_log_info "====检查 $cluster 集群的用户alb===="
  for hr in $(kubectl get hr -n cpaas-system | grep "$cluster-" | awk '{print $1}'); do
    local chart=$(kubectl-with-cluster "global" get hr -n cpaas-system $hr -o jsonpath={.spec.chart})
    if [[ "$chart" != "stable/alauda-alb2" ]]; then
      continue
    fi
    local cpulimit=$(kubectl-with-cluster "global" get hr -n cpaas-system $hr -o jsonpath={.spec.values.resources.limits.cpu})
    if [[ -z "$cpulimit" ]]; then
      _car_log_err "====$cluster 集群的用户alb $hr 的hr中没有设置cpulimit 请检查===="
    fi
    if [[ "$cpulimit" == *"000m"* ]]; then
      _car_log_err "====$cluster 集群的用户alb $hr 的hr中cpulimit格式不正确 请检查===="
    fi
  done
)

function check_alb_resource() (
  if [[ ! "$target_version" =~ ^v(3\.12\.(0|1|2))$ ]]; then
    _car_log_info "目标升级版本是: $target_version, 无需检查alb的resource"
    return
  fi
  _car_check_defualt_alb "global"
  _car_check_user_alb "global"

  for cluster in $(list-running-cluster | grep -v global); do
    _car_check_defualt_alb "$cluster"
    _car_check_user_alb "$cluster"
  done
)
