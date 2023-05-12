#!/bin/bash
function kind-e2e-local() (
  go build -o alb-kind-e2e alauda.io/alb2/test/kind/3.12
  export ALB_SUFFIX=ka
  echo $ALB_SUFFIX
  md5sum ./alb-kind-e2e
  # TODO 在本地跑的时候，正常情况下，我们应该重新自己build 镜像和chart，但现在还没做到
  export ALB_KIND_E2E_CHART=$1
  export ALB_CI_ROOT=/tmp/alb-kind
  mkdir -p $ALB_CI_ROOT
  local base="$ALB_CI_ROOT/$ALB_SUFFIX"
  mkdir -p $base
  mv ./alb-kind-e2e $base
  cd $base
  export ALB_TEST_BASE=$base
  ./alb-kind-e2e
)

kind-e2e-local $@
