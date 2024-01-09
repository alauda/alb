#!/bin/bash
# shellcheck disable=SC2155,SC2086
function check_alb() (
  check_alb_ingress_path
  check_alb_ingress_rule
  check_alb_hr
  check_alb_ingress_httpport
  check_alb_project
  check_alb_resource
)

# 检查默认alb的ingress http port在关闭的情况下，是否还有旧的http port 在
# 在3.10之后才会有这个问题
# 本质上要检查的是apprelease和ft

# Check Ingress Port Log Info
function _cip_log_info() {
  log::info "check-alb-ingress-port $*"
  return
}

# Check Ingress Port Log Error
function _cip_log_err() {
  log::err "check-alb-ingress-port $*"
  return
}

function check_alb_ingress_httpport() (
  local version=${target_version#v}
  IFS='.' read -ra VERSION_PARTS <<<"$version"

  local major="${VERSION_PARTS[0]}"
  minor="${VERSION_PARTS[1]}"

  if [ "$major" -lt 3 ]; then
    _cip_log_info "====$target_version 小于v3.10 无需检查===="
    return
  fi
  if [ "$major" -eq 3 ] && [ "$minor" -lt 10 ]; then
    _cip_log_info "====$target_version 小于v3.10 无需检查===="
    return
  fi

  local alb=$(kubectl-get-alb-apprelease-values-with-cluster global loadbalancerName)
  if [[ -z "$alb" ]]; then
    _cip_log_err "无法获取到合法的alb名 请检查 参照文档: pageId=164987763"
    return
  fi
  _cip_log_info "====检查global集群的默认alb $alb 的 ingress http port===="
  local ingressport=$(kubectl-get-alb-apprelease-values-with-cluster global ingressHTTPPort)
  if [[ "$ingressport" = "0" ]]; then
    if [[ "$(kubectl get ft -n cpaas-system $alb-00080 2>/dev/null | grep -c $alb-00080)" != "0" ]]; then
      _cip_log_err "====global的alb $alb 的 ingresshttpport 为0 但仍有ft? 请检查==== 参照文档: pageId=164987763"
    fi
  fi

  for cluster in $(list_running_cluster | grep -v global); do
    local alb=$(kubectl-get-alb-apprelease-values-with-cluster $cluster loadbalancerName)
    _cip_log_info "====检查业务集群'$cluster'的默认alb $alb 的 ingress http port===="
    local ingressport=$(kubectl-get-alb-apprelease-values-with-cluster $cluster ingressHTTPPort)
    if [[ "$ingressport" = "0" ]]; then
      if [[ "$(kubectl_with_cluster $cluster get ft -n cpaas-system $alb-11780 2>/dev/null | grep -c $alb-11780)" != "0" ]]; then
        _cip_log_err "===='$cluster'的alb $alb 的 ingresshttpport 为0 但仍有ft? 请检查==== 参照文档: pageId=164987763"
      fi
    fi
  done
  return
)

function current_is_ge_3_12() {
  local version=${prdb_version#v}
  IFS='.' read -ra VERSION_PARTS <<<"$version"
  local major="${VERSION_PARTS[0]}"
  local minor="${VERSION_PARTS[1]}"
  local patch="${VERSION_PARTS[3]}"
  if [[ "$major" -gt "3" ]]; then
    echo "true"
    return
  fi
  if [[ "$major" -lt "3" ]]; then
    echo "false"
    return
  fi
  if [[ "$minor" -ge 12 ]]; then
    echo "true"
    return
  fi
  echo "false"
}

function check_alb_project() {
  # 在3.12之后,alb不使用hr, 无需检查
  # 在3.12之前,用户可能会只更新了alb上的label,没有更新hr,所以要进行检查
  if [[ "$(current_is_ge_3_12)" == "true" ]]; then
    log::info "当前版本是$prdb_version, 目标升级版本是: $target_version, 无需检查alb 的project"
    return
  fi
  for cluster in $(list_running_cluster); do
    log::info "====检查 cluster $cluster alb 的 project===="
    while read -r hralb; do
      local alb=$(echo "$hralb" | awk -F "|" '{print $1}')
      local project_in_hr=$(echo "$hralb" | awk -F "|" '{print $2}' | xargs)
      local project_in_alb=$(check-project-in-alb $cluster $alb)
      if [[ "$project_in_hr" = "$project_in_alb" ]]; then
        log::info "$cluster alb $alb hr与alb资源上的项目一致, hr资源上的为: $project_in_hr, alb资源上的为: $project_in_alb"
      else
        log::err "$cluster alb $alb hr与alb资源上的项目不一致, hr资源上的为: $project_in_hr, alb资源上的为: $project_in_alb, 请检查 参照文档: pageId=164987763"
      fi
    done < <(check-project-in-global-hr $cluster)
  done
}

function check-project-in-global-hr() {
  local cluster=$1
  while read -r hr; do
    local albname=${hr#"$cluster-"}
    local project=$(get-project-from-hr $hr)
    echo "$albname | $project"
  done < <(get_albhr_for_cluster $cluster)
}

function check-project-in-alb() {
  local CLUSTER=$1
  local alb=$2
  local project=$(kubectl_with_cluster $CLUSTER label --list alb2 -n cpaas-system $alb | grep project | sed 's/.*project.cpaas.io\/\(.*\)=.*/\1/' | sort | xargs)
  echo "$project" | xargs
}

function get-project-from-hr() {
  local hr=$hr
  local hr_yaml=$(kubectl get hr -n cpaas-system $hr -o yaml)
  result=$(echo "$hr_yaml" | awk -v w1="projects:" -v w2="replicas:" 'BEGIN {flag=0} {if ($0 ~ w1) {flag=1} if (flag==1) {print $0} if ($0 ~ w2) {flag=0}}' | tail -n +2 | head -n -1 | awk '{print $2}' | sort)
  echo $result | xargs
}

# 3.12.0 3.12.1 3.12.2 上
# 1. 资源为空时，升级到operator时，会设置默认的资源，但是nginx给的默认资源太小了。所有这里检查下如果没有显示的设置资源，就让他设置
# 2. 资源的cpu的limit的格式不能为x000m 这种格式。
# 上述问题 3.14.0 及其之后已经修复了

# Check Alb Resource Log Info
function _car_log_info() {
  log::info "check-alb-resource $*"
  return
}

# Check Alb Resource Log Error
function _car_log_err() {
  log::err "check-alb-resource $*"
  return
}

function _car_check_defualt_alb() (
  local cluster="$1"
  _car_log_info "====检查 $cluster 集群的默认alb===="

  local cpulimit=$(kubectl-get-alb-apprelease-values-with-cluster global resources.limits.cpu)
  local name=$(kubectl-get-alb-apprelease-values-with-cluster global loadbalancerName)
  if [[ -z "$cpulimit" ]]; then
    _car_log_err "====$cluster 集群默认alb $name 的apprelease中没有设置cpulimit 请检查==== 参照文档: pageId=164987763"
  fi
  if [[ "$cpulimit" = *"000m"* ]]; then
    _car_log_err "====$cluster 集群默认alb $name 的apprelease中cpulimit格式不正确 请检查==== 参照文档: pageId=164987763"
  fi
  return
)

function _car_check_user_alb() (
  local cluster="$1"
  _car_log_info "====检查 $cluster 集群的用户alb===="
  for hr in $(kubectl get hr -n cpaas-system | grep "$cluster-" | awk '{print $1}'); do
    local chart=$(kubectl_with_cluster "global" get hr -n cpaas-system $hr -o jsonpath="{.spec.chart}")
    if [[ "$chart" != "stable/alauda-alb2" ]]; then
      continue
    fi
    local cpulimit=$(kubectl_with_cluster "global" get hr -n cpaas-system $hr -o jsonpath="{.spec.values.resources.limits.cpu}")
    if [[ -z "$cpulimit" ]]; then
      _car_log_err "====$cluster 集群的用户alb $hr 的hr中没有设置cpulimit 请检查==== 参照文档: pageId=164987763"
    fi
    if [[ "$cpulimit" = *"000m"* ]]; then
      _car_log_err "====$cluster 集群的用户alb $hr 的hr中cpulimit格式不正确 请检查==== 参照文档: pageId=164987763"
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

  for cluster in $(list_running_cluster | grep -v global); do
    _car_check_defualt_alb "$cluster"
    _car_check_user_alb "$cluster"
  done
)

function kubectl-get-alb-apprelease-values-with-cluster() {
  local cluster=$1
  local path=$2
  kubectl_with_cluster $cluster get apprelease -n cpaas-system alauda-alb2 -o jsonpath="{.status.charts.acp/chart-alauda-alb2.values.$path}"
}

function check_alb_ingress_path() (
  # 升级到3.8-3.12.2
  local version=${target_version#v}
  IFS='.' read -ra VERSION_PARTS <<<"$version"
  local major="${VERSION_PARTS[0]}"
  local minor="${VERSION_PARTS[1]}"
  local patch="${VERSION_PARTS[3]}"
  if [[ ! "$major" = "3" ]]; then
    return
  fi
  if [[ "$minor" -lt 8 ]] || [[ "$minor" -gt 12 ]]; then
    return
  fi
  if [[ "$minor" -eq 12 ]] || [[ "$patch" -gt 3 ]]; then
    return
  fi

  log::info "check-alb-ingress-path in global"
  check_alb_ingress_path_in_cluster "global"
  # 1. 检查ingres的path是否有  path:/ 的
  for cluster in $(list_running_cluster | grep -v global); do
    log::info "check-alb-ingress-path in $cluster"
    check_alb_ingress_path_in_cluster "$cluster"
  done
)

function check_alb_ingress_path_in_cluster() (
  local cluster=$1
  local count=$(kubectl_with_cluster $cluster get ingress -A -o yaml | grep -c "path:\s*/\s*$")
  if [[ "$count" -ne 0 ]]; then
    log::err "check-alb-ingress-path 在集群 $cluster 发现$count 个 path: /的ingress 请检查 参照文档: pageId=164987763"
  fi
)

function check_alb_ingress_rule() {
  # 检查alb的rule的ingress是否还在 在3.12之前检查就行
  local version=${prdb_version#v}
  IFS='.' read -ra VERSION_PARTS <<<"$version"
  local major="${VERSION_PARTS[0]}"
  local minor="${VERSION_PARTS[1]}"
  local patch="${VERSION_PARTS[3]}"
  if [[ ! "$major" = "3" ]]; then
    return
  fi
  if [[ "$minor" -gt 12 ]]; then
    return
  fi
  check_alb_ingress_rule_in_cluster "global"
  for cluster in $(list_running_cluster | grep -v "global"); do
    check_alb_ingress_rule_in_cluster "$cluster"
  done
}

function check_alb_ingress_rule_in_cluster() {
  local cluster=$1
  log::info "check-alb-ingress-rule in $cluster"
  local all_rule_source=$(kubectl_with_cluster $cluster get rule -n cpaas-system -l "alb2.cpaas.io/source-type=ingress" -o yaml | grep -E "^\s*source:$" -A 3)
  local all_rule_source_ingress_name=$(echo "$all_rule_source" | grep name: | awk '{print $2}' | sort -u)
  local all_rule_label_ingress_name=$(kubectl_with_cluster $cluster get rule -n cpaas-system -l "alb2.cpaas.io/source-type=ingress" -o yaml | grep -E "^\s*alb2\.cpaas\.io/source-name:\s*" | awk '{print $2}' | awk -F '.' '{print $1}' | sort -u)
  local all_ingress=$(kubectl_with_cluster $cluster get ingress -A | tail -n+2 | awk '{print $2}' | sort -u)
  local all_rule_ingress=$(
    cat <<EOF
$all_rule_source_ingress_name
$all_rule_label_ingress_name
EOF
  )
  for ing in $all_rule_ingress; do
    if [[ -z "$ing" ]]; then
      continue
    fi
    if ! echo "$all_ingress" | grep -q $ing; then
      log::err "check-alb-ingress-rule: 集群 $cluster 的rule的ingress $ing 不存在. 请检查 参照文档: pageId=164987763"
    fi
  done
  return
}

function check_alb_hr() {
  # 检查hr的alb是否还在 在3.12之前检查就行
  local version=${prdb_version#v}
  IFS='.' read -ra VERSION_PARTS <<<"$version"
  local major="${VERSION_PARTS[0]}"
  local minor="${VERSION_PARTS[1]}"
  local patch="${VERSION_PARTS[3]}"
  if [[ ! "$major" = "3" ]]; then
    return
  fi
  if [[ "$minor" -gt 12 ]]; then
    return
  fi

  check_alb_hr_in_cluster "global"
  for cluster in $(list_running_cluster | grep -v "global"); do
    check_alb_hr_in_cluster "$cluster"
  done
}

function check_alb_hr_in_cluster() {
  # 检查alb的hr是否还在
  local cluster=$1
  log::info "check-alb-hr in $cluster"
  while read -r hr; do
    log::info "check hr $hr"
    local albname=${hr#"$cluster-"}
    if ! kubectl_with_cluster $cluster get alb2 -n cpaas-system $albname >/dev/null; then
      log::err "check-alb-hr $cluster 集群的alb $albname hr 还在但是alb不在了 请检查 参照文档: pageId=164987763"
    fi
  done < <(get_albhr_for_cluster $cluster)
  return
}

function get_albhr_for_cluster() {
  local cluster=$1
  kubectl get hr -n cpaas-system -o jsonpath='{range .items[*]}c {.spec.clusterName} c @ {.spec.chart} @ {.metadata.name}{"\n"}{end}' | grep "stable/alauda-alb2" | grep "c $cluster c" | awk -F '@' '{print $3}' | sort
}
