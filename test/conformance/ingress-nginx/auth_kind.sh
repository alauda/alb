#!/bin/bash

function auth-note() {
  # https://github.com/kubernetes/ingress-nginx/blob/main/docs/user-guide/nginx-configuration/annotations.md#authentication
  source ./scripts/alb-dev-actions.sh
  export LUACOV=true
  rm -rf ./luacov*
  alb-nginx-test $PWD/template/t/e2e/auth_test/auth_test.t
  alb-nginx-luacov-summary | grep auth

  helm upgrade --install ingress-nginx ingress-nginx --set controller.image.digest= --set controller.admissionWebhooks.enabled=false --set controller.image.pullPolicy=Never --repo https://kubernetes.github.io/ingress-nginx --namespace ingress-nginx --create-namespace
  return
}

function auth-ingress-nginx-conformance() (
  local chart=$1
  auth-kind $chart

  kind get kubeconfig --name=auth >~/.kube/auth
  export KUBECONFIG=~/.kube/auth
  ginkgo -v ./test/conformance/ingress-nginx
)

function auth-kind() (
  local chart="$1"

  unset KUBECONFIG
  kind-create-1.28.0 auth
  kind get kubeconfig --name=auth >~/.kube/auth
  cp ~/.kube/auth ~/.kube/config

  local alb_image=$(auth-get-alb-image $chart)
  echo "alb image |$alb_image|"

  auth-init-alb-operator $chart
  auth-init-ingress-nginx auth
  auth-init-echo-resty $alb_image
  local IP=$(kubectl get po -n cpaas-system -l service_name=alb2-auth -o wide --no-headers | awk '{print $6}')
  auth-init-alb-ingress
  auth-init-ingress-nginx-ingress
  # ingress-nginx
  curl http://$IP/echo
  # alb
  curl http://$IP:11180/echo
)

function auth-get-alb-image() (
  local chart=$1
  if [[ ! -d "./alauda-alb2" ]]; then
    helm-pull $chart
  fi
  local tag=$(cat ./alauda-alb2/values.yaml | yq .global.images.alb2.tag)
  local alb_image=registry.alauda.cn:60080/acp/alb2:$tag
  echo "$alb_image"
)

function auth-init-alb-operator() (
  local chart="$1"
  if [[ ! -d "./alauda-alb2" ]]; then
    echo "pull chart $chart"
    helm-pull $chart
  fi

  local alb_image=$(_load_alb_image_in_cur_kind ./alauda-alb2 auth)
  #   local alb_image="registry.alauda.cn:60080/acp/alb2:v3.19.0-fix.590.31.gf3ed81db-feat-acp-38937"
  echo "alb image $alb_image"
  helm upgrade --install --namespace cpaas-system --create-namespace --debug alauda-alb --set operator.albImagePullPolicy=IfNotPresent --set defaultAlb=false -f ./alauda-alb2/values.yaml ./alauda-alb2
  local alb=$(
    cat <<EOF
apiVersion: crd.alauda.io/v2beta1
kind: ALB2
metadata:
    name: auth
    namespace: cpaas-system
spec:
    address: "127.0.0.1"  # the ip address of node where alb been deployed
    type: "nginx" 
    config:
        ingressHTTPPort: 11180
        ingressHTTPSPort: 11443
        networkMode: host
        loadbalancerName: auth
        projects:
        - ALL_ALL
        replicas: 1
EOF
  )
  echo "$alb" | kubectl apply -f -
)

function auth-init-echo-resty() (
  local alb_image=$1
  local echo_resty=$(
    cat <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: echo-resty-config
data:
  nginx-config: |
    worker_processes  1;
    daemon off;
    pid nginx.pid;
    env POD_NAME;

    events {
        worker_connections  1024;
    }

    http {
        access_log  /dev/stdout  ;
        error_log   /dev/stdout  info;
        server {
            listen 18880;
            listen [::]:18880;
            location / {
              content_by_lua_block {
                      local h, err = ngx.req.get_headers()
                      if err ~=nil then
                          ngx.say("err: "..tostring(err))
                      end
                      for k, v in pairs(h) do
                          ngx.say(tostring(k).." : "..tostring(v))
                      end
                      ngx.say("pod ".. os.getenv("POD_NAME").." http client-ip "..ngx.var.remote_addr.." client-port "..ngx.var.remote_port.." server-ip "..ngx.var.server_addr.." server-port "..ngx.var.server_port)
              }
            }
        }
    }
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: echo-resty
  labels:
    k8s-app: echo-resty
spec:
  replicas: 1 
  selector:
    matchLabels:
      k8s-app: echo-resty
  template:
    metadata:
      labels:
        k8s-app: echo-resty
    spec:
      hostNetwork: true
      terminationGracePeriodSeconds: 60
      containers:
      - name: echo-resty
        image: {{IMAGE}} 
        env:
          - name: POD_NAME
            valueFrom:
              fieldRef:
                apiVersion: v1
                fieldPath: metadata.name
        resources:
            requests:
              memory: "500Mi"
              cpu: "250m"
            limits:
              memory: "500Mi"
              cpu: "1"
        volumeMounts:
          - name: config-volume
            mountPath: /etc/nginx
        command:
         - sh
         - -c
         - 'mkdir -p /alb/app && cd /alb/app && nginx -p \$PWD -c /etc/nginx/nginx.conf -e /dev/stdout'
        ports:
        - containerPort: 18880
      volumes:
        - name: config-volume
          configMap:
            name: echo-resty-config
            items:
            - key: nginx-config
              path: nginx.conf
---
apiVersion: v1
kind: Service
metadata:
  name: echo-resty
  labels:
    k8s-app: echo-resty
spec:
  ipFamilies:
  - IPv4
  ipFamilyPolicy: PreferDualStack
  ports:
  - name: http
    port: 18880
    targetPort: 18880
  selector:
    k8s-app: echo-resty
EOF
  )
  local echo_resty=$(echo "$echo_resty" | sed "s@{{IMAGE}}@$alb_image@g")
  echo "$echo_resty" | kubectl apply -f -
)

function auth-init-alb-ingress() (
  local echo_ingress=$(
    cat <<EOF
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: echo-alb
spec:
  rules:
  - http:
      paths:
      - path: /echo
        pathType: Prefix
        backend:
          service:
            name: echo-resty
            port:
              number: 18880
EOF
  )

  echo "$echo_ingress" | kubectl apply -f -
)

function auth-init-ingress-nginx() (
  local cluster=$1
  helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx
  helm repo update

  kind load docker-image registry.k8s.io/ingress-nginx/controller:v1.11.3 --name $cluster
  helm upgrade --install ingress-nginx ingress-nginx/ingress-nginx \
    --version 4.11.3 \
    --set controller.hostNetwork=true \
    --set controller.image.digest= \
    --set controller.admissionWebhooks.enabled=false \
    --set controller.image.pullPolicy=Never \
    --namespace ingress-nginx \
    --create-namespace

)

function auth-init-ingress-nginx-ingress() (
  local echo_ingress=$(
    cat <<EOF
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: echo-ingress-nginx
spec:
  ingressClassName: nginx
  rules:
  - http:
      paths:
      - path: /echo
        pathType: Prefix
        backend:
          service:
            name: echo-resty
            port:
              number: 18880
EOF
  )

  echo "$echo_ingress" | kubectl apply -f -
)

function alb-load-chart-image-in-kind() (
  local chart=$1
  local cluster=$2
  mkdir ./.temp-chart
  cd ./.temp-chart
  helm-pull $chart
  ls
  cd ../
  rm -rf ./.temp-chart
)

function _load_alb_image_in_cur_kind() (
  cd $1
  local cluster=$2
  local tag=$(cat ./values.yaml | yq .global.images.alb2.tag)
  local alb_image=registry.alauda.cn:60080/acp/alb2:$tag
  local alb_nginx_image=registry.alauda.cn:60080/acp/alb-nginx:$tag
  (
    docker pull $alb_image
    docker pull $alb_nginx_image
    kind load docker-image $alb_image --name $cluster
    kind load docker-image $alb_nginx_image --name $cluster
  ) >/dev/null
  echo registry.alauda.cn:60080/acp/alb2:$tag
)

function loop() (
  alb-static-build
  md5sum $PWD/bin/alb
  local alb_pod=$(kubectl get po -n cpaas-system --no-headers | grep auth | awk '{print $1}')
  kubectl cp $PWD/bin/alb cpaas-system/$alb_pod:/alb/ctl/alb -c alb2
  #   kubectl cp $PWD/template/nginx/lua cpaas-system/$alb_pod:/alb/nginx/luax -c nginx
  #   ./bin/tools/dirhash ./template/nginx/lua
  #   kubectl exec -n cpaas-system $alb_pod -c nginx -- sh -c 'rm -rf /alb/nginx/lua && mv /alb/nginx/luax /alb/nginx/lua &&  /alb/tools/dirhash /alb/nginx/lua/'
)
