#!/bin/bash

function kind-e2e-local() (
  go test -c alauda.io/alb2/test/kind/e2e
  mv ./test.test ./alb-kind-e2e
  export ALB_SUFFIX=ka
  echo $ALB_SUFFIX
  md5sum ./alb-kind-e2e
  export ALB_KIND_E2E_CHART=v3.13.0-alpha.11
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
