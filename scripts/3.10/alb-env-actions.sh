#!/bin/bash

# install kind kubectl sed cp python3 first
function install-envtest {
  echo "install envtest"
  if [[ ! -d "/usr/local/kubebuilder" ]]; then
    export K8S_VERSION=1.21.2
    curl -sSLo envtest-bins.tar.gz "https://go.kubebuilder.io/test-tools/${K8S_VERSION}/$(go env GOOS)/$(go env GOARCH)"
    # TODO need sudo permissions
    mkdir -p /usr/local/kubebuilder
    tar -C /usr/local/kubebuilder --strip-components=1 -zvxf envtest-bins.tar.gz
    rm envtest-bins.tar.gz
  fi
  if [[ ! "$PATH" =~ "/usr/local/kubebuilder" ]]; then
    echo "you need add /usr/local/kubebuilder to you PATH"
  fi
}

function alb-init-kind-env {
  # how to use
  # . ./scripts/alb-dev-actions.sh;CHART=$PWD/chart KIND_NAME=test-alb  KIND_2_NODE=true alb-init-kind-env
  # . ./scripts/alb-dev-actions.sh; KIND_NAME=alb-38 CHART=build-harbor.alauda.cn/acp/chart-alauda-alb2:v3.8.0-alpha.3 alb-init-kind-env
  # . ./scripts/alb-dev-actions.sh;OLD_ALB=true KIND_VER=v1.19.11 KIND_NAME=alb-342-lowercase CHART=harbor-b.alauda.cn/acp/chart-alauda-alb2:v3.4.2 alb-init-kind-env
  if [ "$DEBUG" = "true" ]; then
    set -x
  fi
  if [ ! -d $ALB_ACTIONS_ROOT ]; then
    echo "could not find $ALB_ACTIONS_ROOT"
    exit
  fi
  temp=~/.temp
  echo $ALB_ACTIONS_ROOT
  local chart=${CHART-"$ALB_ACTIONS_ROOT/../chart"}
  #   local kindName=${KIND_NAME-"kind-alb-${RANDOM:0:5}"}
  local kindName=${KIND_NAME-"alb-dev"}
  local kindVersion=${KIND_VER-"v1.22.2"}
  local kind2node=${KIND_2_NODE-"false"}
  local kindImage="kindest/node:$kindVersion"
  echo $chart
  echo $kindName
  echo $kindImage
  echo $kind2node
  echo $temp
  rm -rf $temp/$kindName
  mkdir -p $temp/$kindName
  generate_kind_config "$kind2node"
  echo "$KINDCONFIG" >$temp/$kindName/kindconfig.yaml
  echo "$KINDCONFIG"
  cat $temp/$kindName/kindconfig.yaml
  kind delete cluster --name $kindName
  cd $temp/$kindName

  _initKind $kindName $kindImage $temp/$kindName/kindconfig.yaml

  local nginx="build-harbor.alauda.cn/3rdparty/alb-nginx:v1.22.0"
  _makesureImage $nginx $kindName
  #init echo-resty yaml
  local echoRestyPath=$temp/$kindName/echo-resty.yaml
  cp "$ALB_ACTIONS_ROOT/yaml/echo-resty.yaml" $echoRestyPath

  sed -i -e "s#{{nginx-image}}#$nginx#" $echoRestyPath
  kubectl apply -f $echoRestyPath
  echo "init echo resty ok"

  # # init ip-provider yaml
  # local ipProviderPath=./.tmp/ip-provider.yaml
  # cp "./yaml/ip-provider.yaml" $ipProviderPath
  # kubectl apply -f $ipProviderPath

  alb-deploy-chart-in-kind $chart $kindName
  echo "init alb"
  init-alb
  tmux select-pane -T $kindName # set tmux pane title
  cd -
}

function alb-deploy-chart-in-kind {
  # helm uninstall alb-dev -n cpaas-system
  # init chart
  local chart=$1
  local kindName=$2
  echo "chart is $chart"
  if [ -d $chart ]; then
    echo "cp alb chart " $chart
    rm -rf ./alauda-alb2
    cp -r $chart ./alauda-alb2
  else
    echo "fetch alb chart " $chart
    helm-chart-export $chart
  fi
  ls ./alauda-alb2
  if [[ $? -ne 0 ]]; then return; fi

  local base="registry.alauda.cn:60080"
  cat ./alauda-alb2/values.yaml
  for im in $(cat ./alauda-alb2/values.yaml | yq eval -o=j | jq -cr '.global.images[]'); do
    local repo=$(echo $im | jq -r '.repository' -)
    local tag=$(echo $im | jq -r '.tag' -)
    local image="$base/$repo:$tag"
    echo "image is $image"
    echo "load image $image to $kindName"
    _makesureImage $image $kindName
  done

  _makesureImage "registry.alauda.cn:60080/ops/alpine:3.16" $kindName
  sed -i 's/imagePullPolicy: Always/imagePullPolicy: Never/g' ./alauda-alb2/templates/deployment.yaml
  kubectl create ns cpaas-system

  if [ "$OLD_ALB" = "true" ]; then
    echo "init crd(old)"
    kubectl apply -f $ALB_ACTIONS_ROOT/yaml/crds/v1beta1/
  else
    echo "init crd(new)"
    kubectl apply -f $ALB_ACTIONS_ROOT/yaml/crds/v1/
    kubectl apply -R -f ./alauda-alb2/crds/
  fi
  echo "helm dry run start"
  read -r -d "" OVERRIDE <<EOF
loadbalancerName: alb-dev
replicas: 1
global:
  labelBaseDomain: cpaas.io
  namespace: cpaas-system
  registry:
    address: registry.alauda.cn:60080
project: ALL_ALL
EOF
  echo "$OVERRIDE" >./alauda-alb2/override.yaml

  helm-alauda install -n cpaas-system \
    -f ./alauda-alb2/values.yaml \
    -f ./alauda-alb2/override.yaml \
    alb-dev \
    ./alauda-alb2 \
    --dry-run --debug
  echo "helm dry run over"

  echo "helm install"
  helm-alauda install -n cpaas-system \
    -f ./alauda-alb2/values.yaml \
    -f ./alauda-alb2/override.yaml \
    alb-dev \
    ./alauda-alb2
}

function init-alb {
  alb-gen-ft 8080 default echo-resty http 80 http
  alb-gen-ft 8443 default echo-resty https 443 https
  alb-gen-ft 8081 default echo-resty tcp 80 tcp
  if [ -z "$OLD_ALB" ]; then
    alb-gen-ft 8553 default echo-resty udp 53 udp
  fi
}

function alb-build-docker-and-update-chart() (
  docker build -t build-harbor.alauda.cn/acp/alb2:local -f ./Dockerfile .
  local arg=""
  if [ -n "$HTTP_PROXY" ]; then
    arg="--build-arg https_proxy=$HTTP_PROXY --build-arg http_proxy=$HTTP_PROXY --network=host"
  fi
  local nginx="docker build $arg -t build-harbor.alauda.cn/acp/alb-nginx:local -f ./alb-nginx/Dockerfile ."
  echo "$nginx"
  eval $nginx
  docker tag build-harbor.alauda.cn/acp/alb2:local registry.alauda.cn:60080/acp/alb2:local
  docker tag build-harbor.alauda.cn/acp/alb-nginx:local registry.alauda.cn:60080/acp/alb-nginx:local
  yq -i '.global.images.alb2.tag="local"' ./deploy/chart/alb/values.yaml
  yq -i '.global.images.nginx.tag="local"' ./deploy/chart/alb/values.yaml
)

function _initKind {
  local kindName=$1
  local kindImage=$2
  local config=$3
  echo "init kind $kindName $kindImage"
  http_proxy="" https_proxy="" all_proxy="" HTTPS_PROXY="" HTTP_PROXY="" ALL_PROXY=""
  kind create cluster --name $kindName --image=$kindImage --config $config

  # TODO fixme kind node notready when set nf_conntrack_max
  kubectl get configmaps -n kube-system kube-proxy -o yaml | sed -r 's/maxPerCore: 32768/maxPerCore: 0/' | kubectl replace -f -
  kubectl create clusterrolebinding lbrb --clusterrole=cluster-admin --serviceaccount=cpaas-system:default
  echo "init kind ok $kindName"
}

function _init_required_crd {
  if [ "$OLD_ALB" = "true" ]; then
    echo "apply feature crd (old)"
    kubectl apply -f $ALB_ACTIONS_ROOT/yaml/crds/v1beta1/
    return
  fi
  kubectl apply -f $ALB_ACTIONS_ROOT/yaml/crds/v1/
}

function _makesureImage {
  local image=$1

  local kindName=$2
  echo "makesureImage " $image $kindName
  if [[ "$(docker images -q $image 2>/dev/null)" == "" ]]; then
    echo "image not exist, pull it $image"
    docker pull $image
  fi
  kind load docker-image $image --name $kindName
}

function init-alb {
  alb-gen-ft 8080 default echo-resty http 80 http
  alb-gen-ft 8443 default echo-resty https 443 https
  alb-gen-ft 8081 default echo-resty tcp 80 tcp
  if [ -z "$OLD_ALB" ]; then
    alb-gen-ft 8553 default echo-resty udp 53 udp
  fi
}

function generate_kind_config {
  local kind2node=$1
  if [[ "$kind2node" == "true" ]]; then
    read -r -d "" KINDCONFIG <<EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
- role: worker
- role: worker
EOF
  else
    read -r -d "" KINDCONFIG <<EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
EOF
  fi
}
