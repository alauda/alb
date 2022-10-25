#!/bin/bash

source $ALB/scripts/alb-env-actions.sh
source $ALB/scripts/alb-test-actions.sh
source $ALB/scripts/alb-lint-actions.sh
source $ALB/scripts/alb-build-actions.sh

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
  $ALB/scripts/rule-gen.py $alb $ft $backendNs $backendSvc
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

function alb-build-iamge {
  docker build -f ./Dockerfile . -t alb-dev
}

function alb-kind-build-and-replace {
  make static-build
  md5sum ./bin/alb
  local alb_pod=$(kubectl get po -A | grep alb-dev | awk '{print $2}' | tr -d '\n')
  echo "alb pod $alb_pod"
  kubectl cp $PWD/bin/alb cpaas-system/"$alb_pod":/alb/alb
  tmux-send-key-to-pane alb 'C-c' 'md5sum ./alb/alb;sleep 3s; /alb/alb 2>&1  | tee alb.log' 'C-m'
}

function alb-patch-gateway-hostname() {
  jsonpatch=$(
    cat <<EOF
[{"op": "replace", "path": "/spec/listeners/0/hostname", "value":"$(od -vAn -N2 -tu2 </dev/urandom | sed 's/ //g').com"}]
EOF
  )
  echo "$jsonpatch"
  kubectl patch gateway -n alb-test g1 --type=json -p="$jsonpatch"
  kubectl get gateway -n alb-test g1 -o yaml
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

function alb-replace-in-pod {
  alb-build-static
  local pod=$(kubectl get po -n cpaas-system | grep alb | awk '{print $1}')
  kubectl cp $PWD/bin/alb cpaas-system/$pod:/alb/alb
  md5sum ./bin/alb
  kubectl exec -it -n cpaas-system $pod -- sh -c 'md5sum /alb/alb'
}
