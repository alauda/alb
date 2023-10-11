#!/bin/bash

# 检查默认alb的ingress http port在关闭的情况下，是否还有旧的http port 在
# 在3.10之后才会有这个问题
# 本质上要检查的是apprelease和ft

# Check Ingress Port Log Info
function _cip_log_info() {
  log::info "check-alb-ingress-port $@"
  return
}

# Check Ingress Port Log Error
function _cip_log_err() {
  log::err "check-alb-ingress-port $@"
  return
}

function check_alb_ingress_httpport() (
  local version=${target_version#v}
  IFS='.' read -ra VERSION_PARTS <<<"$version"

  local major="${VERSION_PARTS[0]}"
  minor="${VERSION_PARTS[1]}"

  if [ "$major" -lt 3 ] || ([ "$major" -eq 3 ] && [ "$minor" -lt 10 ]); then
    _cip_log_info "====$target_version 小于v3.10 无需检查===="
    return
  fi
  local alb=$(kubectl-get-alb-apprelease-values-with-cluster global loadbalancerName)
  if [[ -z "$alb" ]]; then
    _cip_log_err "无法获取到合法的alb名 请检查"
    return
  fi
  _cip_log_info "====检查global集群的默认alb $alb 的 ingress http port===="
  local ingressport=$(kubectl-get-alb-apprelease-values-with-cluster global ingressHTTPPort)
  if [[ "$ingressport" == "0" ]]; then
    if [[ "$(kubectl get ft -n cpaas-system $alb-00080 2>/dev/null | grep $alb-00080 | wc -l)" != "0" ]]; then
      _cip_log_err "====global的alb $alb 的 ingresshttpport 为0 但仍有ft? 请检查===="
    fi
  fi

  for cluster in $(list-running-cluster | grep -v global); do
    local alb=$(kubectl-get-alb-apprelease-values-with-cluster $cluster loadbalancerName)
    _cip_log_info "====检查业务集群'$cluster'的默认alb $alb 的 ingress http port===="
    local ingressport=$(kubectl-get-alb-apprelease-values-with-cluster $cluster ingressHTTPPort)
    if [[ "$ingressport" == "0" ]]; then
      if [[ "$(kubectl-with-cluster $cluster get ft -n cpaas-system $alb-11780 2>/dev/null | grep $alb-11780 | wc -l)" != "0" ]]; then
        _cip_log_err "===='$cluster'的alb $alb 的 ingresshttpport 为0 但仍有ft? 请检查===="
      fi
    fi
  done
  return
)
