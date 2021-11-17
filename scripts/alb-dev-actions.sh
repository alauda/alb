#!/bin/bash

# kind kubectl sed cp python3 mkdir
unset HTTP_PROXY
unset HTTPS_PROXY
unset http_proxy
unset https_proxy
unset ALL_PROXY
unset all_proxy

# export ALB_ACTIONS_ROOT by yourself.
# export ALB_ACTIONS_ROOT=$NS_HOME/share/alauda/alb-actions

alb-test-qps() {
	local defaultKindName=kind-alb-${RANDOM:0:5}
	local defaultAlbImage="build-harbor.alauda.cn/acp/alb2:v3.6.0"
	local defaultNginxImage="build-harbor.alauda.cn/3rdparty/alb-nginx:v3.6.1"

	local kindName=${1-$defaultKindName}
	local albImage=${2-$defaultAlbImage}
	local nginxImage=${3-$defaultNginxImage}
	echo $kindName
	echo $albImage
	echo $nginxImage
	alb-init-kind-env $kindName $albImage $nginxImage

	alb-gen-rule $kindName alb-dev alb-dev-8080 default echo-resty > rule.json
	kubectl apply -f ./rule.json
	local kindIp=$(docker inspect -f '{{range.NetworkSettings.Networks}}{{.IPAddress}}{{end}}' $kindName-control-plane|tr -d '\n')
	echo "kind ip" $kindIp
	wait-curl-success  http://$kindIp:8080/rule-last
	qps=$(ab-qps http://$kindIp:8080/rule-last)
	echo "500 rule" qps $qps $now $albImage $nginxImage
	now=$(date "+%Y%m%d-%H:%M:%S")
}


ab-qps() {
	url=$1
	echo $url
	# TODO use wrk to make sure it run at least 1m
	ab -n 10000 -c 100  $url |grep  "Requests per second"|awk '{print $4}'	
}

alb-gen-rule() {
	local kindName=$1	
	local alb=$2	
	local ft=$3	
	local backendNs=$4	
	local backendSvc=$5
	local ruleFile=/tmp/$kindName.$ft.rule.json
	$ALB_ACTIONS_ROOT/rule-gen.py $alb $ft  $backendNs $backendSvc
}

wait-curl-success() {
	local url=$1
	while true; do
		if curl --fail $url ; then
			echo "success"
			break
		else
			echo "fail"
		fi
		sleep 1s
	done
}

alb-gen-ft() {
	local port=$1
	local backendNs=$2
	local backendSvc=$3

cat <<EOF | kubectl apply -f  -
apiVersion: crd.alauda.io/v1
kind: Frontend
metadata:
  labels:
    alb2.alauda.io/name: alb-dev
    project.alauda.io/ALL_ALL: "true"
  name: alb-dev-$port 
  namespace: cpaas-system
spec:
  backendProtocol: HTTP
  port: $port
  protocol: http
  serviceGroup:
    services:
      - name: $backendSvc
        namespace: $backendNs
        port: 80
        weight: 100
    session_affinity_attribute: ""
    session_affinity_policy: ""
EOF
}

alb-gen-ft-alot() {
	local start=$1
	local end=$2
	local testft=${3-false}

	for i in {$start..$end..1};do 
	echo $i; 
	alb-gen-ft $i default echo-resty;
	start_time=$(date +%s.%6N)  
	if [ "$testft" = "true" ]; then
		wait-curl-success http://172.18.0.2:$i/ping
	fi
	end_time=$(date +%s.%6N) 
	elapsed=$(echo "scale=6; $end_time - $start_time" | bc) 
	echo $elapsed;done
}

alb-gen-hr() {
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

echo $YAML |kubectl apply -f -
}
alb-init-kind-env() {
	# 初始化一个装有alb的kind k8s集群，默认装有echo-resty,配置80端口的ft默认路由为echo-resty
	local defaultKindName=kind-alb-${RANDOM:0:5}
	local defaultAlbImage="build-harbor.alauda.cn/acp/alb2:v3.6.0"
	local defaultNginxImage="build-harbor.alauda.cn/3rdparty/alb-nginx:v3.6.1"

	local kindName=${1-$defaultKindName}
	local albImage=${2-$defaultAlbImage}
	local nginxImage=${3-$defaultNginxImage}
	local lbName="alb-dev"
	local ftPort="8080"
	
	local globalNs="cpaas-system"
	# local debugLog="true"
	local debugLog="false"
	echo $kindName $albImage $nginxImage
    _initKind $kindName
	_makesureImage $albImage $kindName
	_makesureImage $nginxImage $kindName
	_makesureImage "alpine:latest" $kindName


	mkdir -p /tmp/$kindName

	# init echo-resty yaml
	local echoRestyPath=/tmp/$kindName/echo-resty.yaml
	cp "$ALB_ACTIONS_ROOT/yaml/echo-resty.yaml" $echoRestyPath

	sed -i -e "s#{{alb-image}}#$albImage#" $echoRestyPath
	kubectl apply -f $echoRestyPath
	echo "init echo resty ok"

	# # init ip-provider yaml
	# local ipProviderPath=./.tmp/ip-provider.yaml
	# cp "./yaml/ip-provider.yaml" $ipProviderPath
	# kubectl apply -f $ipProviderPath

	kubectl create ns cpaas-system
	kubectl apply -f $ALB_ACTIONS_ROOT/yaml/crds.yaml
    # init instance.yaml which will install alb into kind
    local templatePath="$ALB_ACTIONS_ROOT/yaml/instance.template.yaml"
	local instancePath=/tmp/$kindName/instance-${lbName}.yaml
	cp $templatePath $instancePath
	sed -i -e "s#{{LbName}}#$lbName#" $instancePath
	sed -i -e "s#{{AlbImage}}#$albImage#" $instancePath
	sed -i -e "s#{{NginxImage}}#$nginxImage#" $instancePath
	sed -i -e "s#{{GlobalNs}}#$globalNs#" $instancePath
	sed -i -e "s#{{FtPort}}#$ftPort#" $instancePath
	sed -i -e "s#{{FtUid}}#$lbName#" $instancePath
	# # verbose debug log if we want
	if [ "$debugLog" = "true" ]
	then
		sed -i -e "s#{{DebugLogPlaceHolder}}#error_log /var/log/nginx/debug.log debug;#" $instancePath
	else
		# delete this placeholder
		sed -i -e "s#{{DebugLogPlaceHolder}}##" $instancePath
	fi
	kubectl apply -f $instancePath
}

_initKind() {
	local kindName=$1
	kind delete cluster --name $kindName
	local kindImage="kindest/node:v1.16.15"
	http_proxy="" https_proxy="" all_proxy="" HTTPS_PROXY="" HTTP_PROXY="" ALL_PROXY=""  kind create cluster --name $kindName --image=$kindImage

	# TODO fixme kind node notready when set nf_conntrack_max
	kubectl get configmaps -n kube-system kube-proxy -o yaml|sed   -r 's/maxPerCore: 32768/maxPerCore: 0/'| kubectl replace -f -
	kubectl create clusterrolebinding lbrb --clusterrole=cluster-admin --serviceaccount=cpaas-system:default
	echo "init kind ok $kindName"
}

_makesureImage() {
	local image=$1
	local kindName=$2
	echo "makesureImage " $image $kindName
	if [[ "$(docker images -q $image 2> /dev/null)" == "" ]]; then
		docker pull $image 
	fi
	kind load docker-image $image --name $kindName
}
