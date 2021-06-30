#!/bin/bash

set -e # exit when error
set -x # echo on
# require
# kind kubectl sed cp python3 mkdir
unset HTTP_PROXY
unset HTTPS_PROXY
unset http_proxy
unset https_proxy
unset ALL_PROXY
unset all_proxy

initKind() {
	local kindName=$1
	kind delete cluster --name $kindName
	http_proxy="" https_proxy="" all_proxy="" HTTPS_PROXY="" HTTP_PROXY="" ALL_PROXY=""  kind create cluster --name $kindName --image=kindest/node:v1.16.15

	# kind node notready when set nf_conntrack_max
 	kubectl get configmaps -n kube-system kube-proxy -o yaml|sed   -r 's/maxPerCore: 32768/maxPerCore: 0/'| kubectl replace -f -
	kubectl apply -f ./crds.yaml
	kubectl create clusterrolebinding lbrb --clusterrole=cluster-admin --serviceaccount=cpaas-system:default
	kubectl create ns cpaas-system
	echo "init kind ok $kindName"
}


makesureImage() {
	local image=$1
	local kindName=$2

	if [[ "$(docker images -q $image 2> /dev/null)" == "" ]]; then
		docker pull $image 
	fi
	kind load docker-image $image --name $kindName
}

initBackend() {
	local kindName=$1
	makesureImage  yannrobert/docker-nginx $kindName
	kubectl create ns alb-wc
	kubectl apply -f ./echo-resty.yaml
}

abqps() {
	url=$1
	# TODO use wrk to make sure it run atleast 1m
	ab -n 10000 -c 100  $url |grep  "Requests per second"|awk '{print $4}'	
}


instance() {
	local lbName=$1
	local lbImage=$2
	local globalNs=$3
	local ftPort=$4
	local ftUid=$5
	local templatePath=$6
	local kindName=$7
	# local debugLog="true"
	local debugLog="false"

	makesureImage $lbImage $kindName

	mkdir -p ./.tmp

	local instancePath=./.tmp/instance-${lbName}.yaml
	cp $templatePath $instancePath

	sed -i -e "s#{{LbName}}#$lbName#" $instancePath 
	sed -i -e "s#{{LbImage}}#$lbImage#" $instancePath 
	sed -i -e "s#{{GlobalNs}}#$globalNs#" $instancePath
	sed -i -e "s#{{FtPort}}#$ftPort#" $instancePath
	sed -i -e "s#{{FtUid}}#$lbName#" $instancePath
	# verbose debug log if we want
	if [ "$debugLog" = "true" ]
	then
		sed -i -e "s#{{DebugLogPlaceHolder}}#error_log /var/log/nginx/debug.log debug;#" $instancePath
	else
		# delete this placeholder
		sed -i -e "s#{{DebugLogPlaceHolder}}##" $instancePath
	fi	
	kubectl apply -f $instancePath

	local ruleFile=./.tmp/$lbName-$ftPort.rule.json
	./rule-gen.py $lbName $lbName-$ftPort $ftUid  "alb-wc" "echo-resty" $globalNs > $ruleFile
	kubectl apply -f $ruleFile

	# wait util alb init ok
	sleep 60s

	local kindIp=$(docker inspect -f '{{range.NetworkSettings.Networks}}{{.IPAddress}}{{end}}' $kindName-control-plane|tr -d '\n')
	echo "kind ip" $kindIp
	res=$(curl -s http://$kindIp:$ftPort/rule-499)
	if [ "$res" = "Hello, world!" ]; then
    	echo "Strings are equal"
	else
    	echo "Strings are not equal"
		exit -1
	fi

	qps=$(abqps http://$kindIp:$ftPort/rule-499)
	now=$(date "+%Y%m%d-%H:%M:%S")
	echo "$now $lbImage $qps"
	echo "$now $lbImage $qps" >> qps.out

	# TODO take debug log out, need to pin the name of alb pod

	echo "over"
}

dotest() {
	local image=$1
	local name=$2
	
	initKind $name
	initBackend $name
	
	GlobalNs="cpaas-system"
	TemplateYaml="./instance.template.yaml"

	instance $name $image $GlobalNs "12345" "ft-12345-uid" $TemplateYaml $name
}

compare() {
	LeftImage=$1
	RightImage=$2

	kindNameA="k-16-alb-test-a"
	kindNameB="k-16-alb-test-b"
	

	initKind $kindNameA
	initBackend $kindNameA
	instance "alb-1" $LeftImage $GlobalNs "12345" "ft-12345-uid" $TemplateYaml $kindNameA

	initKind $kindNameB
	initBackend $kindNameB
	instance "alb-2" $RightImage $GlobalNs "12345" "ft-12345-uid" $TemplateYaml $kindNameB
	echo "compare over"
}
