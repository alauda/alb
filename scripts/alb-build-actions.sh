#!/bin/bash

function alb-static-build() {
  set -x
  rm ./bin/alb
  rm ./bin/operator
  CC=/usr/bin/musl-gcc CGO_ENABLED=1 go build -v -buildmode=pie -ldflags '-w -s -linkmode=external -extldflags=-Wl,-z,relro,-z,now,-static' -v -o ./bin/operator alauda.io/alb2/cmd/operator
  CC=/usr/bin/musl-gcc CGO_ENABLED=1 go build -v -buildmode=pie -ldflags '-w -s -linkmode=external -extldflags=-Wl,-z,relro,-z,now,-static' -v -o ./bin/alb alauda.io/alb2/cmd/alb
  CC=/usr/bin/musl-gcc CGO_ENABLED=1 go build -v -buildmode=pie -ldflags '-w -s -linkmode=external -extldflags=-Wl,-z,relro,-z,now,-static' -v -o ./bin/migrate/init-port-info alauda.io/alb2/migrate/init-port-info
  CC=/usr/bin/musl-gcc CGO_ENABLED=1 go build -v -buildmode=pie -ldflags '-w -s -linkmode=external -extldflags=-Wl,-z,relro,-z,now,-static' -v -o ./bin/tools/tweak_gen alauda.io/alb2/cmd/utils/tweak_gen
  CC=/usr/bin/musl-gcc CGO_ENABLED=1 go build -v -buildmode=pie -ldflags '-w -s -linkmode=external -extldflags=-Wl,-z,relro,-z,now,-static' -v -o ./bin/tools/albctl alauda.io/alb2/cmd/utils/albctl

  md5sum ./bin/alb
  md5sum ./bin/operator
  md5sum ./bin/tools/tweak_gen
  md5sum ./bin/tools/albctl
}

function alb-build-all-docker() {
  alb-static-build
  local tag=$(cat ./deploy/chart/alb/values.yaml | yq -r ".global.images.alb2.tag")
  if [[ -n "$1" ]]; then
    tag=$1
  fi
  docker build --network=host -t registry.alauda.cn:60080/acp/alb2:$tag -f ./Dockerfile.local .
  #   source ./template/actions/dev.actions.sh
  #   ang-build-nginx-image $tag
  #   docker pull registry.alauda.cn:60080/acp/alb-nginx:$tag
}

function alb-build-dev-chart() {
  # 1. build alb binary in host and copy to docker
  CC=/usr/bin/musl-gcc CGO_ENABLED=1 go build -v -buildmode=pie -ldflags '-w -s -linkmode=external -extldflags=-Wl,-z,relro,-z,now,-static' -v -o ./bin/operator alauda.io/alb2/cmd/operator
  CC=/usr/bin/musl-gcc CGO_ENABLED=1 go build -v -buildmode=pie -ldflags '-w -s -linkmode=external -extldflags=-Wl,-z,relro,-z,now,-static' -v -o ./bin/alb alauda.io/alb2/cmd/alb
  repo="local"
  version="v0.0.0"
  docker buildx build --network host -t $repo/acp/alb-nginx:$version --build-arg ALB_NGINX_BASE=registry.alauda.cn:60080/acp/alb-nginx:v3.16.0-beta.7.g9df9f40b -f ./template/Dockerfile.local .
  docker tag $repo/acp/alb-nginx:$version $repo/acp/alb2:$version
  rm -rf ./out
  mkdir -p ./out
  cp -rf ./deploy/chart/alb ./out
  chart_dir=./out/alb
  local values=$(
    cat <<EOF
operator:
  albImagePullPolicy: IfNotPresent
defaultAlb: false
operatorReplicas: 1
operatorDeployMode: "deployment"
global:
  labelBaseDomain: cpaas.io
  namespace: kube-system
  registry:
    address: "$repo"
  images:
    alb2:
      repository: acp/alb2
    nginx:
      repository: acp/alb-nginx
EOF
  )
  cp ./.github/chart/alb/crds/crd.alauda.io_alaudaloadbalancer2.yaml $chart_dir/crds # do not served v1
  echo "$values" >$chart_dir/values.yaml
  yq -i e ".global.images.alb2.tag |= \"$version\"" $chart_dir/values.yaml
  yq -i e ".global.images.nginx.tag |= \"$version\"" $chart_dir/values.yaml
  yq -i e ".version |= \"$version\"" $chart_dir/Chart.yaml
  return
}
