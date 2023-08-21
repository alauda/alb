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

function ang-start() (
  mkdir -p ./.ang/logs
  local alb=$PWD
  local chaos=$alb/template/actions/chaos
  cd ./.ang
  cp $chaos/alb.nginx.conf ./
  ln -s $alb ./alb2
  configmap_to_file $alb/template/tweak
  sed -i '/proxy_send*/d' $alb/template/tweak/http.conf
  sed -i '/proxy_read*/d' $alb/template/tweak/http.conf
  export SYNC_POLICY_INTERVAL=1
  export CLEAN_METRICS_INTERVAL=2592000
  export NEW_POLICY_PATH=$chaos/policy.json
  export DEFAULT_SSL_STRATEGY=NEVER
  export INGRESS_HTTPS_PORT=443
  nginx -p $PWD -c $PWD/alb.nginx.conf
  return
)

function nb-start-alb() {
  return
}
