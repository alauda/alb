#!/bin/bash

function alb-gen-crd-and-client() (
  # alb crd
  controller-gen crd paths=./pkg/apis/alauda/... output:crd:dir=./deploy/chart/alb/crds/
  # gateway crd
  # gateway的crd是直接复制的
  # client
  rm -rf pkg/client

  # use `go get k8s.io/code-generator@v0.24.4-rc.0` to install correspond code_generator first
  chmod a+x $GOPATH/pkg/mod/k8s.io/code-generator@v0.24.4-rc.0/generate-groups.sh
  # NOTE 手动更新一些配置
  ./scripts/update-codegen.sh
)
