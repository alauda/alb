#!/bin/bash

# shellcheck disable=SC2155,SC2086
function check_alb_legacy() (
  log::info "check-alb 开始"
  check_alb_ingress_path
  check_alb_ingress_rule
  check_alb_hr
  check_alb_ingress_httpport
  check_alb_project
  check_alb_resource
  check_alb_configmap
  log::info "check-alb 结束"
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
  done < <(alb_get_hr_for_cluster $cluster)
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

function _car_check_default_alb() (
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
  _car_check_default_alb "global"
  _car_check_user_alb "global"

  for cluster in $(list_running_cluster | grep -v global); do
    _car_check_default_alb "$cluster"
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
  # 检查hr和alb是否一一对应 在3.12之前检查就行
  if [[ "$(_alb_compare_ver $prdb_version "ge" '3.12')" == "true" ]]; then
    return
  fi

  check_alb_hr_in_cluster "global"
  for cluster in $(list_running_cluster | grep -v "global"); do
    check_alb_hr_in_cluster "$cluster"
  done
}

function check_alb_hr_in_cluster() {
  # 检查 hr <-> alb
  local cluster=$1
  log::info "check-alb-hr in $cluster"
  local hrs=$(alb_get_hr_for_cluster $cluster)
  local albs=$(alb_list_user_alb_for_cluster $cluster | xargs -I{} echo $cluster-{} | awk '{print $1}')
  if [[ -z "$hrs" ]] && [[ -z "$albs" ]]; then
    log::info "check-alb-hr no alb in $cluster skip"
    return
  fi
  local all=$(_alb_join_lines "$hrs" "$albs")
  local all_count=$(echo "$all" | sort | uniq -c)
  local diff=$(echo "$all_count" | grep -v "2 " | awk '{print $2}')
  while read -r hr; do
    if [[ -z "$hr" ]]; then
      continue
    fi
    log::err "check-alb-hr $cluster invalid hr and alb |$hr|"
    if ! grep -q "^${hr}$" <<<"$hrs"; then
      log::err "check-alb-hr $cluster 集群的alb $hr alb还在但是hr不在了 请检查 参照文档: pageId=164987763"
    fi
    if ! grep -q "^${hr}$" <<<"$albs"; then
      log::err "check-alb-hr $cluster 集群的alb $hr hr还在但是alb不在了 请检查 参照文档: pageId=164987763"
    fi
  done < <(echo "$diff")
  return
}

function alb_get_hr_for_cluster() {
  local cluster=$1
  kubectl get hr -n cpaas-system -o jsonpath='{range .items[*]}c {.spec.clusterName} c @ {.spec.chart} @ {.metadata.name}{"\n"}{end}' | grep "stable/alauda-alb2" | grep "c $cluster c" | awk -F '@' '{print $3}' | awk '{print $1}'
}

function alb_list_user_alb_for_cluster() {
  local cluster=$1
  kubectl_with_cluster $cluster get alb2 -n cpaas-system --no-headers --ignore-not-found=true | grep -v "cpaas-system" | grep -v "global-alb2" | awk '{print $1}'
}

function check_alb_configmap() (
  mkdir -p ./.alb_check
  local version=${prdb_version#v}
  IFS='.' read -ra VERSION_PARTS <<<"$version"
  local major="${VERSION_PARTS[0]}"
  local minor="${VERSION_PARTS[1]}"
  local patch="${VERSION_PARTS[2]}"
  if [[ "$major" -lt "3" ]]; then
    return
  fi
  if [[ "$minor" -lt "12" ]]; then
    check_alb_configmap_lt_3_12 "global"
    for cluster in $(list_running_cluster | grep -v "global"); do
      check_alb_configmap_lt_3_12 "$cluster"
    done
    return
  fi
  if [[ "$minor" -ge "12" ]]; then
    check_alb_configmap_ge_3_12 "global"
    for cluster in $(list_running_cluster | grep -v "global"); do
      check_alb_configmap_ge_3_12 "$cluster"
    done
    return
  fi
)

function check_alb_configmap_lt_3_12() (
  local cluster="$1"
  log::info "check alb configmap change <3.12 in cluster $cluster"

  while read -r hr; do
    log::info "check alb hr $hr"
    local albname=${hr#"$cluster-"}
    local cm=$(kubectl_with_cluster $cluster get cm -n cpaas-system $albname -o yaml)
    local ver=$(kubectl_with_cluster "global" get hr -n cpaas-system $hr -o jsonpath="{.status.version}")
    local expect_cm_base=$(_alb_chcm_get_expect_cm_base $ver)
    local needbackup="false"

    local cm_backup_dir="$backup_dir/$cluster/alb/check_cm/$albname"
    mkdir -p "$cm_backup_dir"
    for key_file in "./custom/alb/check_cm/cm_map/$expect_cm_base"/*; do
      local key=$(basename $key_file)
      local expect_val=$(cat $key_file)
      local cur_val=$(echo "$cm" | yq ".data.\"$key\"")
      if [[ "$cur_val" != "$expect_val" ]]; then
        echo "$cur_val" >./.alb_check/$ver-$key
        local diff=$(diff -u $key_file ./.alb_check/$ver-$key)
        log::err "check alb hr $hr configmap $albname $key 的值不正确, diff --->| $diff |<--- 请检查 参照文档: pageId=164987763#check_alb_configmap_lt_3_12"
        needbackup="true"
        echo "$diff" >"$cm_backup_dir/$key-diff"
      fi
    done
    if [[ "$needbackup" == "true" ]]; then
      echo "$cm" >"$cm_backup_dir/cur.cm.yaml"
      log::info "check alb hr $hr configmap $albname, 当前configmap已备份到 $cm_backup_dir/cur.cm.yaml"
    fi
  done < <(alb_get_hr_for_cluster $cluster)
  return
)

function check_alb_configmap_ge_3_12() (
  local cluster=$1
  log::info "check alb configmap change >=3.12 in cluster $cluster"

  while read -r alb; do
    log::info "check alb configmap change >=3.12 in cluster $cluster $alb"
    local cm_patch=$(kubectl_with_cluster $cluster get alb2.v2beta1.crd -n cpaas-system $alb -o jsonpath="{.spec.config.overwrite.configmap}")
    if [[ -n "$cm_patch" ]]; then
      log::err "check alb $alb configmap patch,此 alb cm 被patch 请检查 参照文档: pageId=164987763#check_alb_configmap_ge_3_12"
    fi
  done < <(kubectl_with_cluster $cluster get alb2.v2beta1.crd -n cpaas-system --no-headers | awk '{print $1}')
  return
)

function _alb_chcm_get_expect_cm_base() (
  local ver=$1
  ver=${ver#v}
  IFS='.' read -ra VERSION_PARTS <<<"$ver"
  local major="${VERSION_PARTS[0]}"
  local minor="${VERSION_PARTS[1]}"
  local patch="${VERSION_PARTS[2]}"
  if [[ "$major" == "3" ]] && [[ "$minor" == "0" ]]; then
    echo "v3.0"
    return
  fi
  if [[ "$major" == "3" ]] && [[ "$minor" == "2" ]]; then
    echo "v3.0"
    return
  fi
  if [[ "$major" == "3" ]] && [[ "$minor" == "4" ]]; then
    echo "v3.4"
    return
  fi
  if [[ "$major" == "3" ]] && [[ "$minor" == "6" ]] && [[ "$patch" -lt "9" ]]; then
    echo "v3.6.0"
    return
  fi
  if [[ "$major" == "3" ]] && [[ "$minor" == "6" ]] && [[ "$patch" -ge "9" ]]; then
    echo "v3.6.9"
    return
  fi
  if [[ "$major" == "3" ]] && [[ "$minor" == "8" ]] && [[ "$patch" -lt "13" ]]; then
    echo "v3.8.0"
    return
  fi
  if [[ "$major" == "3" ]] && [[ "$minor" == "8" ]] && [[ "$patch" -ge "13" ]]; then
    echo "v3.8.13"
    return
  fi
  if [[ "$major" == "3" ]] && [[ "$minor" == "10" ]]; then
    echo "v3.10"
    return
  fi
)

function check_alb_port_project() (
  local cluster=$1
  if [[ "$(_alb_compare_ver $prdb_version "ge" '3.12')" == "true" ]]; then
    log::info "check_alb_port_project 当前版本是$prdb_version, 目标升级版本是: $target_version, 无需检查alb 的port project"
    return
  fi
  log::info "check_alb_port_project 检查alb 的port project"
  if [[ "$(_alb_compare_ver $target_version "lt" '3.8.2')" == "true" ]]; then
    # 每次升级后手动更新configmap
    check_alb_port_project_notice_update_cm $cluster
    return
  fi

  if [[ "$(_alb_compare_ver $target_version "ge" '3.8.2')" == "true" ]]; then
    # 检查 cm和port project是否一致 保证hr上port-project正常,即可
    check_alb_port_project_notice_update_hr $cluster
    return
  fi
  log::info "check_alb_port_project ok"
  return
)

function check_alb_port_project_notice_update_hr() {
  local cluster=$1
  log::info "check_alb_port_project/hr 检查 $cluster 集群的alb的port project"
  local port_from_cm=$(_alb_list_port_project_from_cm $cluster)
  if [[ -z "$port_from_cm" ]]; then
    return
  fi
  while read -r line; do
    local alb=$(echo "$line" | awk '{print $1}')
    local ports_in_cm=$(echo "$line" | awk '{print $2}')
    local ports_in_hr=$(_alb_get_port_project_from_hr $cluster $alb)
    if [[ "$ports_in_cm" == "$ports_in_hr" ]]; then
      continue
    fi

    local cm_backup_dir="$backup_dir/$cluster/alb/check_alb_port_project/$alb"
    local cm_backup_yaml="$cm_backup_dir/$alb-port-info.yaml"
    mkdir -p "$cm_backup_dir"
    kubectl_with_cluster $cluster get cm -n cpaas-system $alb-port-info -o yaml >$cm_backup_yaml
    kubectl_with_cluster global get hr -n cpaas-system $cluster-$alb -o yaml >$cm_backup_dir/hr.yaml

    log::err "check_alb_port_project 集群 $cluster 的 alb $alb 的端口项目信息 configmap 与 hr不一致 cm: $ports_in_cm hr: $ports_in_hr  请更新hr $cluster-$alb. 参照文档: pageId=164987763#check_alb_port_project"
  done < <(echo "$port_from_cm")
}

function check_alb_port_project_notice_update_cm() {
  local cluster=$1
  log::info "check_alb_port_project/cm 检查 $cluster 集群的alb的port project"

  local port_from_cm=$(_alb_list_port_project_from_cm $cluster)
  if [[ -z "$port_from_cm" ]]; then
    return
  fi
  while read -r line; do
    local alb=$(echo "$line" | awk '{print $1}')
    local ports=$(echo "$line" | awk '{print $2}')

    local cm_backup_dir="$backup_dir/$cluster/alb/check_alb_port_project/$alb"
    local cm_backup_yaml="$cm_backup_dir/$alb-port-info.yaml"
    mkdir -p "$cm_backup_dir"
    kubectl_with_cluster $cluster get cm -n cpaas-system $alb-port-info -o yaml >$cm_backup_yaml
    kubectl_with_cluster global get hr -n cpaas-system $cluster-$alb -o yaml >$cm_backup_dir/hr.yaml
    log::err "check_alb_port_project 集群 $cluster 的 alb $alb 的端口项目信息 $ports 将在升级后丢失，请升级后手动更新configmap $alb-port-info. backup $cm_backup_yaml 参照文档: pageId=164987763#check_alb_port_project"
  done < <(echo "$port_from_cm")
}

function _alb_list_port_project_from_cm() {
  local cluster=$1
  while read -r alb; do
    local ports=$(kubectl_with_cluster $cluster get cm -n cpaas-system $alb-port-info --ignore-not-found=true -o jsonpath='{.data.range}')
    if [[ -z "$ports" ]] || [[ "$ports" == "[]" ]]; then
      continue
    fi
    echo $alb $ports
  done < <(alb_list_user_alb_for_cluster $cluster | awk '{print $1}')
}

function _alb_get_port_project_from_hr() {
  local cluster=$1
  local alb=$2
  kubectl_with_cluster global get hr -n cpaas-system $cluster-$alb -o jsonpath="{.spec.values.portProjects}"
  return
}

function _alb_join_lines() {
  local all=$(
    cat <<EOF
$1
$2
EOF
  )
  echo "$all"
}

function _alb_compare_ver() {
  local left=${1#v}
  local op=$2 # >=(ge) <=(le) >(gt) <(lt) ==(eq)
  local right=${3#v}
  if [[ "$op" =~ "e" ]] && [[ "$left" == "$right" ]]; then
    echo "true"
    return
  fi

  if [[ "$op" == "eq" ]]; then
    if [[ "$left" == "$right" ]]; then
      echo "true"
    fi
    echo "false"
    return
  fi

  if [[ "$op" =~ "g" ]]; then
    local lower=$(_alb_join_lines "$left" "$right" | sort -V | head -n 1)
    if [[ "$lower" == "$right" ]]; then
      echo "true"
    else
      echo "false"
    fi
    return
  fi

  if [[ "$op" =~ "l" ]]; then
    local lower=$(_alb_join_lines "$left" "$right" | sort -V | head -n 1)
    if [[ "$lower" == "$left" ]]; then
      echo "true"
    else
      echo "false"
    fi
    return
  fi
  echo "false"
}

function check_alb_default_http() (
  local cluster=$1
  if [[ "$cluster" == "global" ]]; then
    return
  fi

  if [[ "$(_alb_compare_ver $prdb_version "ge" '3.18')" == "true" ]] || [[ "$(_alb_compare_ver $target_version "lt" '3.18')" == "true" ]]; then
    return
  fi

  local not_ingress_rule=$(kubectl_with_cluster $cluster get rule -n cpaas-system -l "alb2.cpaas.io/source-type!=ingress,alb2.cpaas.io/frontend=cpaas-system-11780,alb2.cpaas.io/name=cpaas-system" --no-headers --ignore-not-found=true)
  if [[ $(echo "$not_ingress_rule" | wc -l) != "0" ]]; then
    log::err "check_alb_default_http 集群 $cluster 的默认alb cpaas-system,发现用户自建规则 |$not_ingress_rule|.请检查.  参照文档: pageId=164987763#check_alb_default_http"
    return
  fi
)
