#!/bin/bash

function alb-cleanup() {
  while read -r alb; do
    echo "delete alb $alb"
    kubectl patch alb2 $alb -n cpaas-system -p '{"metadata":{"finalizers":null}}' --type=merge
    kubectl delete alb2 $alb -n cpaas-system
  done < <(kubectl get alb2 -n cpaas-system | tail -n +2 | awk '{print $1}')
  helm list | grep alb | awk '{print $1}' | xargs -I{} helm uninstall {}
}

function alb-run-operator-in-local() (
  export USE_KUBE_CONFIG=$KUBECONFIG
  export LEADER_NS=cpaas-system
  export ALB_IMAGE="registry.alauda.cn:60080/acp/alb2:v3.14.1"
  export NGINX_IMAGE="registry.alauda.cn:60080/acp/alb-nginx:v3.14.1"
  export VERSION="v3.14.1"
  export LABEL_BASE_DOMAIN="cpaas.io"
  export IMAGE_PULL_POLICY="IfNotPresent"

  kubectl delete leases.coordination.k8s.io -n cpaas-system alb-operator

  go run alauda.io/alb2/cmd/operator 2>&1 | tee ./gw.log
)
