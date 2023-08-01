#!/bin/bash

function ang-build-nginx-image() {
  local tag="local"
  if [[ -n "$1" ]]; then
    tag=$1
  fi
  echo "build alb-nginx local"
  docker build -t build-harbor.alauda.cn/acp/alb-nginx:$tag -f ./template/Dockerfile .
  docker tag build-harbor.alauda.cn/acp/alb-nginx:$tag registry.alauda.cn:60080/acp/alb-nginx:$tag
}

function ang-local-test() {
  if [ -d ./.t ]; then
    sudo rm -rf ./.t
  fi
  docker run --user root -it -e SKIP_INSTALL_NGINX_TEST_DEP=true -v $PWD/.t:/t/ -v $PWD/template/nginx/:/alb/nginx/ -v $PWD:/acp-alb-test build-harbor.alauda.cn/acp/alb-nginx-tester:local sh -c 'cd /acp-alb-test ;/acp-alb-test/scripts/nginx-test.sh'
}
