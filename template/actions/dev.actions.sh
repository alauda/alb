#!/bin/bash

function ang-build-image() {
  docker build -t build-harbor.alauda.cn/acp/alb-nginx:local -f ./alb-nginx/Dockerfile .
  docker build -t build-harbor.alauda.cn/acp/alb-nginx-tester:local -f ./alb-nginx/nginx-test-runner.dockerfile .
}

function ang-local-test() {
  if [ -d ./.t ]; then
    sudo rm -rf ./.t
  fi
  docker run --user root -it -e SKIP_INSTALL_NGINX_TEST_DEP=true -v $PWD/.t:/t/ -v $PWD/template/nginx/:/alb/nginx/ -v $PWD:/acp-alb-test build-harbor.alauda.cn/acp/alb-nginx-tester:local sh -c 'cd /acp-alb-test ;/acp-alb-test/scripts/nginx-test.sh'
}
