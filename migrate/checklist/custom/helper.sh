#!/bin/bash

function kubectl-with-cluster() {
  local CLUSTER=$1
  shift
  if [[ "$CLUSTER" == "global" ]]; then
    echo "$(kubectl $@)"
    return
  fi
  local CLUSER_ADD=$(kubectl get clusters.platform.tkestack.io $CLUSTER -o jsonpath={.status.addresses[0].host})
  local CLUSER_PORT=$(kubectl get clusters.platform.tkestack.io $CLUSTER -o jsonpath={.status.addresses[0].port})
  local CLUSER_CC_NAME=$(kubectl get clusters.platform.tkestack.io $CLUSTER -o jsonpath={.spec.clusterCredentialRef.name})
  local CLUSTER_TOKEN=$(kubectl get cc $CLUSER_CC_NAME -oyaml | grep token | awk '{print $2}')
  KUBECTL="kubectl --insecure-skip-tls-verify=true --server https://$CLUSER_ADD:$CLUSER_PORT --token $CLUSTER_TOKEN"
  echo "$($KUBECTL $@)"
}

function kubectl-get-alb-apprelease-values-with-cluster() {
  local cluster=$1
  local path=$2
  kubectl-with-cluster $cluster get apprelease -n cpaas-system alauda-alb2 -o jsonpath={.status.charts.acp/chart-alauda-alb2.values.$path}
}

function list-running-cluster() {
  kubectl get clusters.platform.tkestack.io | grep Running | awk '{print $1}'
}
