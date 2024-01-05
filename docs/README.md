# Alauda Load Balancer v2

# how to deploy in kind
1. create kind cluster
2. download chart ( TODO create helm repo)
3. load image in kind cluster
4. helm install alb-operator 
```
helm install alb-operator -f ./values.yaml  --set operator.albImagePullPolicy=IfNotPresent --set defaultAlb=false --set global.namespace=kube-system --set operatorDeployMode=deployment  .
```
# how to create and use a alb as ingress controller
```bash
cat <<EOF | kubectl apply -f -
apiVersion: crd.alauda.io/v2beta1
kind: ALB2
metadata:
    name: alb-demo
    namespace: kube-system
spec:
    address: "172.20.0.5"  # the ip address of node where alb been deployed
    type: "nginx" 
    config:
        networkMode: host
        loadbalancerName: alb-demo
        nodeSelector:
          alb-demo: "true"
        projects:
        - ALL_ALL
        replicas: 1
EOF
```
prepare the demo app
```bash
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
```
now you could `curl http://${ip}`
# more advance config
## ft and rule
除了ingress之外，可以通过创建ft和rule，来指定alb的路由规则
```yaml
apiVersion: crd.alauda.io/v1
kind: Frontend
metadata:
  labels:
    alb2.cpaas.io/name: alb-demo # 必须要通过label指定这个ft属于那个alb
  name: alb-demo-00080
  namespace: kube-system
spec:
  backendProtocol: ""   # 转发到的后端的协议
  certificate_name: ""  # 如果是https，希望配置默认证书，通过这个字段设置。格式为
  port: 80              # 这个端口本身的协议
  protocol: http
---
apiVersion: crd.alauda.io/v1
kind: Rule
metadata:
  labels:
    alb2.cpaas.io/frontend: alb-demo-00080  # 必须要通过label指定这个rule属于那个ft
    alb2.cpaas.io/name: alb-demo            # 必须要通过label指定这个rule属于那个alb
  name: alb-demo-00080-topu
  namespace: kube-system
spec:
  backendProtocol: ""
  certificate_name: ""
  corsAllowHeaders: ""
  corsAllowOrigin: ""
  description: default/hello-world:80:0
  dsl: '[{[[STARTS_WITH /]] URL }]'         # stringilze的匹配规则，用来被搜索
  dslx:                                     # 匹配规则，详细配置见 TODO 匹置规则
  - type: URL
    values:
    - - STARTS_WITH
      - /
  enableCORS: false
  priority: 5                              # 规则优先级，越小越会优先匹配
  redirectCode: 0
  redirectURL: ""
  rewrite_base: /
  serviceGroup:
    services:
    - name: hello-world
      namespace: default
      port: 80
      weight: 100
  url: /
```
## ingress 
### ingress with other port
### supported annotation
#### alb only 
##### rewrite request
headers_add 只会在没有这个header是才添加
alb.ingress.cpaas.io/rewrite-request
```yaml
alb.ingress.cpaas.io/rewrite-request: |
{"headers_remove":["h1"],"headers":{"a":"b"},"headers_add":{"aa","bb"}}
```
##### rewrite response
alb.ingress.cpaas.io/rewrite-response
```yaml
alb.ingress.cpaas.io/rewrite-response: |
{"headers_remove":["h1"],"headers":{"a":"b"},"headers_add":{"aa","bb"}}
```

#### compatiable with ingress-nginx
##### rewrite
	nginx.ingress.kubernetes.io/rewrite-target
##### cors
	nginx.ingress.kubernetes.io/enable-cors
	nginx.ingress.kubernetes.io/cors-allow-headers
	nginx.ingress.kubernetes.io/cors-allow-origin
##### backend
	nginx.ingress.kubernetes.io/backend-protocol
##### redirect
	nginx.ingress.kubernetes.io/temporal-redirect
	nginx.ingress.kubernetes.io/permanent-redirect
##### vhost
	nginx.ingress.kubernetes.io/upstream-vhost"
## alb 配置
### 容器网络模式
alb默认是以主机网络部署的，这样的好处是可以通过节点ip直接访问，缺点是每个alb只能独占节点，或者需要手动管理alb的端口。
同样alb也支持容器网络模式部署，并通过lbsvc来提供对外访问的能力
```yaml
apiVersion: crd.alauda.io/v2beta1
kind: ALB2
metadata:
    name: alb-demo
    namespace: kube-system
spec:
    type: "nginx" 
    config:
        networkMode: container           # 部署的alb pod使用容器网络模式
        vip:
            enableLbSvc: true            # 自动创建一个loadbalancer类型的svc，并且将分配给svc的地址当作alb的地址
        loadbalancerName: alb-demo
        nodeSelector:
          alb-demo: "true"
        projects:
        - ALL_ALL
        replicas: 1
```
### gatewayapi
alb原生支持gatewayapi，只要创建gateway的时候指定gatewayclass为`exclusive-gateway`即可
```yaml
apiVersion: gateway.networking.k8s.io/v1alpha2
kind: Gateway
metadata:
  name: g1 
  namespace: g1 
spec:
  gatewayClassName:  exclusive-gateway
  listeners:
  - name: http
    port: 80
    protocol: HTTP
    allowedRoutes:
      namespaces:
        from: All
```