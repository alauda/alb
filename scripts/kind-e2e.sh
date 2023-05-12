#!/bin/bash
set -x
# require env ALB_KIND_E2E_CHART. optional env ALB_KIND_E2E_BRANCH
go build -o alb-kind-e2e alauda.io/alb2/test/kind/3.12
export ALB_SUFFIX=$(date '+%s')
echo $ALB_SUFFIX
md5sum ./alb-kind-e2e
sshpass -p 07Apples@ ssh -o StrictHostKeyChecking=no root@192.168.66.2 ls
sshpass -p 07Apples@ scp -o StrictHostKeyChecking=no ./alb-kind-e2e root@192.168.66.2:/root/alb-kind-e2e-$ALB_SUFFIX

env

function test-in-ci() {
  echo "in remote ci"
  export ALB_EXTRA_CRS=/root/alb_extra_crd
  export ALB_CI_ROOT=/root/ci
  mkdir -p /root/ci
  local base="/root/ci/alb-base-$ALB_SUFFIX"
  mkdir -p $base
  cd $base
  export ALB_TEST_BASE=$base
  env
  mv /root/alb-kind-e2e-$ALB_SUFFIX ./
  md5sum ./alb-kind-e2e-$ALB_SUFFIX
  ./alb-kind-e2e-$ALB_SUFFIX
}

sshpass -p 07Apples@ ssh -o StrictHostKeyChecking=no root@192.168.66.2 <<EOF
    $(typeset -f test-in-ci)
    export ALB_KIND_E2E_BRANCH=$ALB_KIND_E2E_BRANCH
    export ALB_KIND_E2E_CHART=$ALB_KIND_E2E_CHART
    export ALB_SUFFIX=$ALB_SUFFIX
    test-in-ci 
EOF
