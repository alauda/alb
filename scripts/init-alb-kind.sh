# create alb environment in localhost

# k8s 1.16.5
## init Kind
## kind delete cluster --name k-1.16.5
## http_proxy="" https_proxy="" all_proxy="" HTTPS_PROXY="" HTTP_PROXY="" ALL_PROXY="" kind create cluster --name k-1.16.5 --image kindest/node:v1.16.15@sha256:c10a63a5bda231c0a379bf91aebf8ad3c79146daca59db816fb963f731852a99

## init ns

kubectl create ns cpaas-system
kubectl create ns wc

## create imagePullSecrets
kubectl create secret  docker-registry alauda-harbor --docker-server=harbor-b.alauda.cn --docker-username=Cong_Wu --docker-password=djk8332h7mkk2j9ejx8yby4h94vd6laq --docker-email=congwu@alauda.io
kubectl patch serviceaccount default -p "{\"imagePullSecrets\": [{\"name\": \"alauda-harbor\"}]}" 

## init cluster-base charts
cd ../../chart-alauda-cluster-base/chart
## readinessprobe built-int
helm install --debug alauda-cluster-base .

# helm uninstall --debug alauda-cluster-base 
# // bug 1. tracejob 2. readinessprobe 3. built-init
cd -
## init alb2 charts
#  //bug 1. values replicas namespace resgistry 2. deployment podAntiffinity
helm install --debug alauda-alb2   --set-string loadbalancerName=alb-wc --set-string global.registry.address=harbor-b.alauda.cn --set-string global.namespace=cpaas-system --set-string antiAffinityKey="xx" --set-string project=ALL_ALL --set-string replicas=1 .
# helm uninstall --debug alauda-alb2 

## set alb label projects to ALL_ALL which where accept all ingress

kubectl patch deployment -n cpaas-system alb-wc --patch '
spec:
  template:
    spec:
        imagePullSecrets:
        - name: alauda-harbor
'

kubectl label alb2 alb-wc "project.alauda.io/name=ALL_ALL" --overwrite=true --namespace=cpaas-system
## deployment some service to test alb
kubectl apply -f ./scripts/resources/echo-service-1.yaml -n wc
kubectl apply -f ./scripts/resources/echo-service-2.yaml -n wc
## init ingress
kubectl apply -f ./scripts/resources/echo-ingress -n wc

## test loadbalancer via forward

alb=$(kubectl get pod -n cpaas-system|grep alb| awk '{print $1}'| tr -d '\n')
kubectl port-forward -n cpaas-system ${alb}  8080:80 
