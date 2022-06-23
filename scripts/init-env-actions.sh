#!/bin/bash

function alb-init-kind-env {
  # required
  # helm: >= 3.7.0
  # docker: login to build-harbor.alauda.cn
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
  local kindName=${KIND_NAME-"kind-alb-${RANDOM:0:5}"}
  local kindVersion=${KIND_VER-"v1.22.2"}
  local kind2node=${KIND_2_NODE-"false"}
  local kindImage="kindest/node:$kindVersion"
  local nginx="build-harbor.alauda.cn/3rdparty/alb-nginx:20220118182511"
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

  # init chart
  echo "chart is $chart"
  if [ -d $chart ]; then
    echo "cp alb chart " $chart
    cp -r $chart ./alauda-alb2
  else
    echo "fetch alb chart " $chart
    helm-chart-export $chart
  fi
  ls ./alauda-alb2
  if [[ $? -ne 0 ]]; then return; fi

  _initKind $kindName $kindImage $temp/$kindName/kindconfig.yaml

  local base="registry.alauda.cn:60080"
  for im in $(cat ./alauda-alb2/values.yaml | yq eval -o=j | jq -cr '.global.images[]'); do
    local repo=$(echo $im | jq -r '.repository' -)
    local tag=$(echo $im | jq -r '.tag' -)
    local image="$base/$repo:$tag"
    echo "load image $image to $kindName"
    _makesureImage $image $kindName
  done

  _makesureImage "build-harbor.alauda.cn/ops/alpine:3.16" $kindName
  _makesureImage $nginx $kindName

  local lbName="alb-dev"
  local ftPort="8080"

  local globalNs="cpaas-system"
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

  kubectl create ns cpaas-system

  sed -i 's/imagePullPolicy: Always/imagePullPolicy: Never/g' ./alauda-alb2/templates/deployment.yaml
  # alb <=3.7
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
nodeSelector:
  node-role.kubernetes.io/master: ""
loadbalancerName: alb-dev
global:
  labelBaseDomain: cpaas.io
  namespace: cpaas-system
  registry:
    address: registry.alauda.cn:60080
project: all_all
replicas: 1
EOF
  echo "$OVERRIDE" >./override.yaml

  helm install --dry-run --debug alb-dev ./alauda-alb2 --namespace cpaas-system -f ./alauda-alb2/values.yaml -f ./override.yaml
  echo "helm dry run over"

  echo "helm install"

  helm install --debug alb-dev ./alauda-alb2 --namespace cpaas-system -f ./alauda-alb2/values.yaml -f ./override.yaml

  echo "init alb"
  init-alb
  tmux select-pane -T $kindName # set tmux pane title
  cd -
}


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