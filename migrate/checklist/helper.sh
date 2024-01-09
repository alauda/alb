#!bin/bash
##日志函数
function log::err() {
  printf "[$(date +'%Y-%m-%dT%H:%M:%S')]: \033[31mERROR: \033[0m$@\n"
}

function log::info() {
  printf "[$(date +'%Y-%m-%dT%H:%M:%S')]: \033[32mINFO: \033[0m$@\n"
}

function list_running_cluster() {
  kubectl get clusters.platform.tkestack.io | grep Running | awk '{print $1}'
}

function kubectl_with_cluster() {
  local CLUSTER=$1
  shift
  if [[ "$CLUSTER" = "global" ]]; then
    kubectl $*
    return
  fi
  local CLUSER_ADD=$(kubectl get clusters.platform.tkestack.io $CLUSTER -o jsonpath="{.status.addresses[0].host}")
  local CLUSER_PORT=$(kubectl get clusters.platform.tkestack.io $CLUSTER -o jsonpath="{.status.addresses[0].port}")
  local CLUSER_CC_NAME=$(kubectl get clusters.platform.tkestack.io $CLUSTER -o jsonpath="{.spec.clusterCredentialRef.name}")
  local CLUSTER_TOKEN=$(kubectl get cc $CLUSER_CC_NAME -oyaml | grep token | awk '{print $2}')
  KUBECTL="kubectl --insecure-skip-tls-verify=true --server https://$CLUSER_ADD:$CLUSER_PORT --token $CLUSTER_TOKEN"
  $KUBECTL $*
}
