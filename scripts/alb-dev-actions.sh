#!/bin/bash
# give [zmx](https://github.com/woodgear/zmx) a try

# install kind kubectl sed cp python3 first
unset HTTP_PROXY
unset HTTPS_PROXY
unset http_proxy
unset https_proxy
unset ALL_PROXY
unset all_proxy

function install-envtest {
  echo "install envtest"
  if [[ ! -d "/usr/local/kubebuilder" ]]; then
    export K8S_VERSION=1.19.2
    curl -sSLo envtest-bins.tar.gz "https://go.kubebuilder.io/test-tools/${K8S_VERSION}/$(go env GOOS)/$(go env GOARCH)"

    mkdir -p /usr/local/kubebuilder
    tar -C /usr/local/kubebuilder --strip-components=1 -zvxf envtest-bins.tar.gz
    rm envtest-bins.tar.gz
  fi
  if [[ ! "$PATH" =~ "/usr/local/kubebuilder" ]]; then
    echo "you need add /usr/local/kubebuilder to you PATH"
  fi
}

function alb-test-qps {
  local defaultKindName=kind-alb-${RANDOM:0:5}
  local defaultAlbImage="build-harbor.alauda.cn/acp/alb2:v3.6.0"
  local defaultNginxImage="build-harbor.alauda.cn/3rdparty/alb-nginx:20220118182511"

  local kindName=${1-$defaultKindName}
  local albImage=${2-$defaultAlbImage}
  local nginxImage=${3-$defaultNginxImage}
  echo $kindName
  echo $albImage
  echo $nginxImage
  alb-init-kind-env $kindName $albImage $nginxImage

  alb-gen-rule $kindName alb-dev alb-dev-8080 default echo-resty >rule.json
  kubectl apply -f ./rule.json
  local kindIp=$(docker inspect -f '{{range.NetworkSettings.Networks}}{{.IPAddress}}{{end}}' $kindName-control-plane | tr -d '\n')
  echo "kind ip" $kindIp
  wait-curl-success http://$kindIp:8080/rule-last
  qps=$(ab-qps http://$kindIp:8080/rule-last)
  echo "500 rule" qps $qps $now $albImage $nginxImage
  now=$(date "+%Y%m%d-%H:%M:%S")
}

function ab-qps {
  url=$1
  echo $url
  # TODO use wrk to make sure it run at least 1m
  ab -n 10000 -c 100 $url | grep "Requests per second" | awk '{print $4}'
}

function alb-gen-rule {
  local kindName=$1
  local alb=$2
  local ft=$3
  local backendNs=$4
  local backendSvc=$5
  local ruleFile=/tmp/$kindName.$ft.rule.json
  $ALB_ACTIONS_ROOT/rule-gen.py $alb $ft $backendNs $backendSvc
}

function wait-curl-success {
  local url=$1
  while true; do
    if curl --fail $url; then
      echo "success"
      break
    else
      echo "fail"
    fi
    sleep 1s
  done
}

function alb-gen-ft {
  echo "alb-gen-ft"
  local port=$1
  local backendNs=$2
  local backendSvc=$3
  local protocol=$4
  local svcPort=$5
  local backendProtocol=$6
  local apiversion="apiVersion: crd.alauda.io/v1"
  cat <<EOF | kubectl apply -f -
$apiversion
kind: Frontend
metadata:
  labels:
    alb2.cpaas.io/name: alb-dev
    project.cpaas.io/ALL_ALL: "true"
  name: alb-dev-$port-$protocol
  namespace: cpaas-system
spec:
  backendProtocol: $backendProtocol
  port: $port
  protocol: $protocol
  serviceGroup:
    services:
      - name: $backendSvc
        namespace: $backendNs
        port: $svcPort
        weight: 100
    session_affinity_attribute: ""
    session_affinity_policy: ""
EOF
}

function alb-gen-ft-alot {
  local start=$1
  local end=$2
  local testft=${3-false}

  for i in {$start..$end..1}; do
    echo $i
    alb-gen-ft $i default echo-resty
    start_time=$(date +%s.%6N)
    if [ "$testft" = "true" ]; then
      wait-curl-success http://172.18.0.2:$i/ping
    fi
    end_time=$(date +%s.%6N)
    elapsed=$(echo "scale=6; $end_time - $start_time" | bc)
    echo $elapsed
  done
}

function alb-gen-hr {
  #arg-len: 4
  local clusterName=$1
  local hrName=$2
  local address=$3
  local registry=$4
  local version=$5

  read -r -d "" YAML <<EOF
apiVersion: app.alauda.io/v1
kind: HelmRequest
metadata:
  annotations:
    cpaas.io/creator: admin@cpaas.io
  finalizers:
    - captain.cpaas.io
  name: $clusterName-$hrName
  namespace: cpaas-system
spec:
  chart: stable/alauda-alb2
  clusterName: $clusterName
  namespace: cpaas-system
  values:
    address: $address
    displayName: ''
    enablePortProject: false
    global:
      labelBaseDomain: cpaas.io
      namespace: cpaas-system
      registry:
        address: $registry
    loadbalancerName: $hrName
    nodeSelector:
      kubernetes.io/hostname: $address
    projects:
      - ALL_ALL
    replicas: 1
    resources:
      limits:
        cpu: 200m
        memory: 256Mi
      requests:
        cpu: 200m
        memory: 256Mi
  version: $version

EOF
  echo $YAML

  echo $YAML | kubectl apply -f -
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

function alb-build-iamge {
  docker build -f ./Dockerfile . -t alb-dev
}

function alb-kind-build-and-replace {
  make static-build
  md5sum ./bin/alb
  local alb_pod=$(kubectl get po -A | grep alb-dev | awk '{print $2}'| tr -d '\n')
  echo "alb pod $alb_pod" 
  kubectl cp $PWD/bin/alb  cpaas-system/"$alb_pod":/alb/alb
  tmux-send-key-to-pane alb 'C-c' 'md5sum ./alb/alb;sleep 3s; /alb/alb 2>&1  | tee alb.log' 'C-m'
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

  local base="build-harbor.alauda.cn"
  cat ./alauda-alb2/values.yaml
  for im in $(cat ./alauda-alb2/values.yaml | yq eval -o=j | jq -cr '.global.images[]'); do
    local repo=$(echo $im | jq -r '.repository' -)
    local tag=$(echo $im | jq -r '.tag' -)
    local image="$base/$repo:$tag"
    echo "load image $image to $kindName"
    _makesureImage $image $kindName
  done

  _makesureImage "build-harbor.alauda.cn/ops/alpine:3.14.2" $kindName
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

  sed -i "s/alauda-system/cpaas-system/g" ./alauda-alb2/values.yaml
  sed -i "s/replicas: 3/replicas: 1/g" ./alauda-alb2/values.yaml
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
nodeSelector:
  node-role.kubernetes.io/master: ""
EOF
  echo "$OVERRIDE" >./override.yaml

  helm-alauda install -n cpaas-system \
    --set-string loadbalancername=alb-dev \
    --set-string global.labelBaseDomain=cpaas.io \
    --set-string global.registry.address=build-harbor.alauda.cn \
    --set-string project=all_all \
    -f ./alauda-alb2/values.yaml \
    -f ./override.yaml \
    alb-dev \
    ./alauda-alb2 \
    --dry-run --debug
  echo "helm dry run over"

  echo "helm install"
  helm-alauda install -n cpaas-system \
    --set-string loadbalancerName=alb-dev \
    --set-string global.labelBaseDomain=cpaas.io \
    --set-string global.namespace=cpaas-system \
    --set-string global.registry.address=build-harbor.alauda.cn \
    --set-string project=ALL_ALL \
    -f ./alauda-alb2/values.yaml \
    -f ./override.yaml \
    alb-dev \
    ./alauda-alb2

  echo "init alb"
  init-alb
  tmux select-pane -T $kindName # set tmux pane title
  cd -
}

function init-alb {
  alb-gen-ft 8080 default echo-resty http 80 http
  alb-gen-ft 8443 default echo-resty https 443 https
  alb-gen-ft 8081 default echo-resty tcp 80 tcp
  if [ -z "$OLD_ALB" ]; then
    alb-gen-ft 8553 default echo-resty udp 53 udp
  fi
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

function alb-list-e2e-testcase {
  ginkgo -v -noColor -dryRun ./test/e2e | grep 'alb-test-case'
}

function alb-run-one-e2e-test {
	cp ./viper-config.toml ./test/e2e
  local testcase=$( ginkgo -v -noColor -dryRun ./test/e2e | grep 'alb-test-case' | sed -e 's/.*alb-test-case\s*//g' |fzf )
  echo "case is $testcase"
  if [ -z "$testcase" ]; then
    echo "empty case ingore"
    return
  fi
  ginkgo -v=1 -focus "$testcase" ./test/e2e
}

function alb-patch-gateway-hostname() {
jsonpatch=$(cat <<EOF
[{"op": "replace", "path": "/spec/listeners/0/hostname", "value":"$(od -vAn -N2 -tu2 < /dev/urandom|sed 's/ //g').com"}]
EOF
);echo "$jsonpatch"; kubectl patch gateway -n alb-test g1 --type=json -p="$jsonpatch";kubectl get gateway -n alb-test g1  -o yaml
}

function alb-run-all-e2e-test {
  cp ./viper-config.toml ./test/e2e
  while IFS= read -r testcase _; do
    echo "run test $testcase"
    ginkgo -v -focus "$testcase" ./test/e2e
    RESULT=$?
    if [ $RESULT -eq 0 ]; then
      echo success
    else
      echo "run $testcase fail"
      exit 1
    fi
  done < <(ginkgo -v -noColor -dryRun ./test/e2e | grep 'alb-test-case' | sed -e 's/.*alb-test-case\s*//g')
}

function get-alb-images-from-values {
  local base="build-harbor.alauda.cn"
  for im in $(yq eval -o=j ./chart/values.yaml | jq -cr '.global.images[]'); do
    local repo=$(echo $im | jq -r '.repository' -)
    local tag=$(echo $im | jq -r '.tag' -)
    local image="$base/$repo:$tag"
    _makesureImage $image
  done
}

function helm-chart-export {
  local chart=$1
  if helm-alauda chart list 2>&1 | grep $chart 2>&1 >/dev/null; then
    echo "find $chart in local"
  else
    echo "pull $chart"
    helm-alauda chart pull $chart 2>&1 >/dev/null
  fi
  helm-alauda chart export $chart 2>&1 >/dev/null
}

TOUCHED_LUA_FILE=("utils/common.lua" "worker.lua")
function alb-lua-format-check {
  # shellcheck disable=SC2068
  for f in ${TOUCHED_LUA_FILE[@]}; do
    echo check format of $f
    local lua=./template/nginx/lua/$f
    lua-format --check -v $lua
  done
}

function alb-lua-format-format {
  # shellcheck disable=SC2068
  for f in ${TOUCHED_LUA_FILE[@]}; do
    echo format $f
    local lua=./template/nginx/lua/$f
    lua-format -i -v $lua
  done
}

function alb-init-git-hook {
  read -r -d "" PREPUSH <<EOF
#!/bin/bash
set -e

function check-branch-name {
    current_branch=\$(git branch --show-current |tr -d '\n\r')
    if [[ \$current_branch == *acp* ]] ; 
    then
        echo "let's use ACP.."
        exit -1
    fi
}

sudo rm -rf ./alb-nginx/t/servroot # T_T
check-branch-name
make test
make all-e2e-envtest
make lua-test
cd chart
helm lint -f ./values.yaml
EOF
  echo "$PREPUSH" >./.git/hooks/pre-push
  chmod a+x ./.git/hooks/pre-push
}

function alb-go-coverage {
  # copy from https://github.com/ory/go-acc
  touch ./coverage.tmp
  echo 'mode: atomic' >coverage.txt
  go list ./... | grep -v /e2e | grep -v /pkg | xargs -n1 -I{} sh -c 'go test -race -covermode=atomic -coverprofile=coverage.tmp -coverpkg $(go list ./... | grep -v /pkg |grep -v /e2e | tr "\n" ",") {} && tail -n +2 coverage.tmp >> coverage.txt || exit 255' && rm coverage.tmp
  go tool cover -func=./coverage.txt
  go tool cover -html=./coverage.txt -o coverage.html
}

function go-fmt-fix {
  gofmt -w .
}

function go-lint {
  gofmt -l .
  if [ ! "$(gofmt -l .)" = "" ]; then
    echo "go fmt check fail"
    exit 1
  fi
  go vet .../..
}

function go-unit-test {
  if [ -d ./alb-nginx/t/servroot ]; then
      sudo rm -rf ./alb-nginx/t/servroot || true
  fi
  go test -v -coverprofile=coverage-all.out $(go list ./... | grep -v e2e)
}

function alb-test-all-in-ci {
  # the current in ci is sth like /home/xx/xx/acp-alb-test
  set -e # exit on err
  echo alb is $ALB
  echo pwd is $(pwd)
  local START=$(date +%s)
  go-lint
  luacheck ./template/nginx/lua
  alb-lua-format-check
  go-unit-test
  go install github.com/onsi/ginkgo/ginkgo
  mv ~/go/bin/ginkgo /usr/bin
  alb-run-all-e2e-test
  source ./alb-nginx/actions/common.sh
  test-nginx-in-ci
  local END=$(date +%s)
  echo "all-time: " $(echo "scale=3; $END - $START" | bc) "s"
}

function alb-run-local {
  # alb-init-kin-env
  # kubectl scale -n cpaas-system deployment alb-dev --replicas=0   # we want run alb in our self
  rm -rf ./.alb.local
  mkdir -p ./.alb.local
  mkdir -p ./.alb.local/last_status
  mkdir -p ./.alb.local/tweak
  export NAME=alb-dev
  export NAMESPACE=cpaas-system
  export DOMAIN=cpaas.io
  export ALB_STATUSFILE_PARENTPATH=./.alb.local/last_status
  export NGINX_TEMPLATE_PATH=./template/nginx/nginx.tmpl
  export NEW_CONFIG_PATH=./.alb.local/nginx.conf.new
  export OLD_CONFIG_PATH=./.alb.local/nginx.conf
  export NEW_POLICY_PATH=./.alb.local/policy.new
  export ALB_TWEAK_DIRECTORY=./.alb.local/tweak
  export ALB_E2E_TEST_CONTROLLER_ONLY=true
  export USE_KUBECONFIG=true
  export ALB_LOG_EXT=true
  export ALB_LOG_LEVEL=8
  go run main.go
}