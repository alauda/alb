#!/bin/bash
# 生成部署所需要的chart
function alb-deploy-build-alb-and-operator() (
  set -e
  alb-deploy-build-alb-chart
  tree ./deploy/chart/alb
)

function alb-deploy-build-alb-chart() (
  set -e
  set -x
  cd ./deploy
  echo "build alb-chart"
  rm -rf ./chart/alb/crds
  mkdir ./chart/alb/crds
  find ./resource/crds -type f -name "*.yaml" -exec cp {} ./chart/alb/crds \;
  cat ./chart/alb/templates/alb.yaml
  cat ./chart/alb/templates/deploy-csv.yaml
  cat ./chart/alb/templates/deploy-deployment.yaml
)

function alb-gen-crd-and-client() (
  # alb crd
  rm -rf ./deploy/resource/crds/alb
  controller-gen crd paths=./pkg/apis/alauda/... output:crd:dir=./deploy/resource/crds/alb
  # gateway crd
  rm -rf ./deploy/resource/crds/alb/gateway*
  rm -rf ./deploy/resource/crds/gateway_policyattachment
  controller-gen crd paths=./pkg/apis/alauda/gateway/v1alpha1 output:crd:dir=./deploy/resource/crds/gateway_policyattachment
  # client
  rm -rf pkg/client

  # use `go get k8s.io/code-generator@v0.24.4-rc.0` to install correspond code_generator first
  chmod a+x $GOPATH/pkg/mod/k8s.io/code-generator@v0.24.4-rc.0/generate-groups.sh
  ./scripts/update-codegen.sh
)
