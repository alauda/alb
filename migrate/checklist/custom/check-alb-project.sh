#!/bin/bash
function check_alb_project() {
  local same="true"
  if [[ "$target_version" =~ ^v(3\.12\.(0|1))$ ]]; then
    for cluster in $(list-running-cluster); do
      log::info "====检查 cluster $cluster alb 的 project===="
      while read -r hralb; do
        local alb=$(echo "$hralb" | awk -F "|" '{print $1}')
        local project_in_hr=$(echo "$hralb" | awk -F "|" '{print $2}' | xargs)
        local project_in_alb=$(check-project-in-alb $cluster $alb)
        if [[ "$project_in_hr" == "$project_in_alb" ]]; then
          log::info "$cluster alb $alb hr与alb资源上的项目一致, hr资源上的为: $project_in_hr, alb资源上的为: $project_in_alb"
        else
          log::err "$cluster alb $alb hr与alb资源上的项目不一致, hr资源上的为: $project_in_hr, alb资源上的为: $project_in_alb, 请检查"
          same="false"
        fi
      done < <(check-project-in-global-hr $cluster)
    done
  else
    log::info "目标升级版本是: $target_version, 无需检查alb 的project"
  fi
}
function check-project-in-global-hr() {
  local cluster=$1
  while read -r hr; do
    local albname=${hr#"$cluster-"}
    local project=$(get-project-from-hr $hr)
    echo "$albname | $project"
  done < <(kubectl get hr -n cpaas-system -o yaml | grep "chart: stable/alauda-alb2" -B 5 | grep "name:" | awk '{print $2}' | grep $cluster | sort)
}
function check-project-in-alb() {
  local CLUSTER=$1
  local alb=$2
  local project=$(kubectl-with-cluster $CLUSTER label --list alb2 -n cpaas-system $alb | grep project | sed 's/.*project.cpaas.io\/\(.*\)=.*/\1/' | sort | xargs)
  echo "$project" | xargs
}
function get-project-from-hr() {
  local hr=$hr
  local hr_yaml=$(kubectl get hr -n cpaas-system $hr -o yaml)
  result=$(echo "$hr_yaml" | awk -v w1="projects:" -v w2="replicas:" 'BEGIN {flag=0} {if ($0 ~ w1) {flag=1} if (flag==1) {print $0} if ($0 ~ w2) {flag=0}}' | tail -n +2 | head -n -1 | awk '{print $2}' | sort)
  echo $result | xargs
}
