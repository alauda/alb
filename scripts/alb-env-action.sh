#!/bin/bash

function alb-init-kind-env {
  kind-create-1.27.3 a-port
  kubectl apply -R -f ./scripts/yaml/crds/extra/v1
  kubectl create ns cpaas-system
  local tag=$(cat ./deploy/chart/alb/values.yaml | yq -r .global.images.alb2.tag)
  kind-pull-andload-image-in-current registry.alauda.cn:60080/acp/alb2:$tag
  kind-pull-andload-image-in-current registry.alauda.cn:60080/acp/alb-nginx:$tag
  helm install alauda-alb --set operator.albImagePullPolicy=IfNotPresent --set loadbalancerName=alauda-alb --set operatorDeployMode=deployment -f ./deploy/chart/alb/values.yaml ./deploy/chart/alb
}
