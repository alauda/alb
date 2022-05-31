#!/bin/bash

# docker run --network=host -v $PWD:/acp-alb-test -it build-harbor.alauda.cn/3rdparty/alb-nginx-test:20211227214905 sh -c 'cd /acp-alb-test ;/acp-alb-test/scripts/ci.sh'
pwd
set -e
export ALB=$PWD
export ALB_ACTIONS_ROOT=$ALB/scripts
echo $ALB
source ./scripts/alb-dev-actions.sh
alb-test-all-in-ci