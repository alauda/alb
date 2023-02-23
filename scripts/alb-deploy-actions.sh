#!/bin/bash
# 生成部署所需要的chart
function alb-deploy-build-alb-and-operator() (
  set -e
  alb-deploy-install-deps

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
  cat ./chart/alb/templates/csv.yaml
)

function alb-deploy-install-deps() {
  # install operator-sdk bin
  curl -LO https://github.com/mikefarah/yq/releases/latest/download/yq_linux_amd64
  mv ./yq_linux_amd64 /usr/bin/yq
  rm $(which yq) || true
  chmod +x /usr/bin/yq
  echo "download yq ok"
  yq --help
  md5sum /usr/bin/yq
  which yq
}

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
