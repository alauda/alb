#!/bin/bash

source $ALB/scripts/alb-test-actions.sh
source $ALB/scripts/alb-lint-actions.sh
source $ALB/scripts/alb-build-actions.sh
source $ALB/scripts/alb-deploy-actions.sh
source $ALB/template/actions/alb-nginx.sh

function alb-cleanup() {
  while read -r alb; do
    echo "delete alb $alb"
    kubectl patch alb2 $alb -n cpaas-system -p '{"metadata":{"finalizers":null}}' --type=merge
    kubectl delete alb2 $alb -n cpaas-system
  done < <(kubectl get alb2 -n cpaas-system | tail -n +2 | awk '{print $1}')
  helm list | grep alb | awk '{print $1}' | xargs -I{} helm uninstall {}
}
# while true; do sleep 1s; kubectl logs -f -n cpaas-system $(k get po -n cpaas-system|grep lb-1 | awk '{print $1}') -c alb2;done
# while true; do sleep 1s; kubectl logs -f -n cpaas-system $(k get po -n cpaas-system|grep operator | awk '{print $1}') ;done

function alb-kubectl-use-devmode-envtest() {
  export KUBECONFIG=/tmp/alb-test-base/kubectl/kubecfg
}
