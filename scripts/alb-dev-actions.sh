#!/bin/bash

source ./scripts/alb-test-actions.sh
source ./scripts/alb-lint-actions.sh
source ./scripts/alb-build-actions.sh
source ./scripts/alb-deploy-actions.sh
source ./scripts/alb-github-actions.sh
source ./template/actions/alb-nginx.sh

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

function alb-init-kind-env() {
  kubectl create ns cpaas-system
  kubectl apply -R -f ./scripts/yaml/crds/extra/v1
  # TODO remove this it should be done in csv
  kubectl apply -R -f ./deploy/resource/rbac/
  kubectl apply -R -f ./deploy/chart/alb/crds
  helm template --set loadbalancerName=global-alb2 --set global.images.nginx.tag=v3.14.1 --set global.images.alb2.tag=v3.14.1 --set operatorDeployMode=deployment --debug ./deploy/chart/alb -f ./deploy/chart/alb/values.yaml | kubectl apply -f -
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

function alb-sync-to-checklist() {
  local target="$1"
  for p in ./migrate/checklist/custom/*; do
    md5sum $p
    cp $p $target/checklist/custom/
  done
  echo "====="
  for p in $target/checklist/custom/*; do
    md5sum $p
  done
}

function alb-sync-from-checklist() {
  local target="$1"
  for p in $target/checklist/custom/*; do
    md5sum $p
    cp $p ./migrate/checklist/custom/
  done
  echo "====="
  for p in ./migrate/checklist/custom/*; do
    md5sum $p
  done
}
