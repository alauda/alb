#!/bin/bash

function alb-env-init-kind() (
  #   kind-create-1.27.3 a-port
  #   kubectl apply -R -f ./scripts/yaml/crds/extra/v1
  kubectl create ns cpaas-system
  _load_alb_image_in_cur_kind
  k apply -R -f ./deploy/chart/alb/crds/
  helm install --debug alauda-alb --set projects="{ALL_ALL}" --set replicas=1 --set operator.albImagePullPolicy=IfNotPresent --set loadbalancerName=alauda-alb --set operatorDeployMode=deployment -f ./deploy/chart/alb/values.yaml ./deploy/chart/alb
)

function _load_alb_image_in_cur_kind() (
  local tag=$(cat ./deploy/chart/alb/values.yaml | yq .global.images.alb2.tag)
  kind-pull-andload-image-in-current registry.alauda.cn:60080/acp/alb2:$tag
  kind-pull-andload-image-in-current registry.alauda.cn:60080/acp/alb-nginx:$tag
)

function alb-env-init-demo() (
  kind-pull-andload-image-in-current docker.io/crccheck/hello-world:latest
  cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: hello-world
  labels:
    k8s-app: hello-world
spec:
  replicas: 1 
  selector:
    matchLabels:
      k8s-app: hello-world
  template:
    metadata:
      labels:
        k8s-app: hello-world
    spec:
      terminationGracePeriodSeconds: 60
      containers:
      - name: hello-world
        image: docker.io/crccheck/hello-world:latest 
        imagePullPolicy: IfNotPresent
---
apiVersion: v1
kind: Service
metadata:
  name: hello-world
  labels:
    k8s-app: hello-world
spec:
  ports:
  - name: http
    port: 80
    targetPort: 8000
  selector:
    k8s-app: hello-world 
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: hello-world
spec:
  ingressClassName: alauda-alb
  rules:
  - http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: hello-world
            port:
              number: 80
EOF
)
