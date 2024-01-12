# Alauda Load Balancer v2

# how to deploy operator in kind
1. create kind cluster
2. `helm repo add alb https://alauda.github.io/alb/;helm repo update;helm search repo|grep alb`
3. `helm install alb-operator alb/alauda-alb2` 
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
in addition to ingress, routing rules for alb can be configed by ft and rule
```yaml
apiVersion: crd.alauda.io/v1
kind: Frontend
metadata:
  labels:
    alb2.cpaas.io/name: alb-demo # required, indicate the alb to which this ft belongs
  name: alb-demo-00080
  namespace: kube-system
spec:
  backendProtocol: ""   # http|https 
  certificate_name: ""  # $secret_ns/$secret_name
  port: 80              
  protocol: http        # protocol of this ft itself
---
apiVersion: crd.alauda.io/v1
kind: Rule
metadata:
  labels:
    alb2.cpaas.io/frontend: alb-demo-00080  # required, indicate the ft to which this rule belongs
    alb2.cpaas.io/name: alb-demo            # required, indicate the alb to which this rule belongs
  name: alb-demo-00080-topu
  namespace: kube-system
spec:
  backendProtocol: ""                       # as same as ft
  certificate_name: ""                      # as same as ft
  dslx:                                     # 匹配规则，详细配置见 TODO 匹置规则
  - type: URL
    values:
    - - STARTS_WITH
      - /
  enableCORS: false
  priority: 5                              # Rule Prioritization, the smaller it is the more it will match first则优先级，越小越会优先匹配
  serviceGroup:
    services:
    - name: hello-world
      namespace: default
      port: 80
      weight: 100
```
## ingress 
### ingress with other port
### supported annotation
#### alb only 
##### rewrite request
headers_remove: remove the header
headers_add: append to header instead of overwrite it.
headers: set the header
```yaml
alb.ingress.cpaas.io/rewrite-request: |
{"headers_remove":["h1"],"headers":{"a":"b"},"headers_add":{"aa": ["bb","cc"]}}
```
##### rewrite response
as same as rewrite request
```yaml
alb.ingress.cpaas.io/rewrite-response: |
{"headers_remove":["h1"],"headers":{"a":"b"},"headers_add":{"aa": ["bb","cc"]}}
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
## alb
### container network
By default, alb is deployed as a host network, which has the advantage of direct access via node ip, and the disadvantage that each alb can only have exclusive access to the node, or you need to manually manage the alb's ports.
But, alb also supports container network mode deployment and provides external access through lbsvc.
```yaml
apiVersion: crd.alauda.io/v2beta1
kind: ALB2
metadata:
    name: alb-demo
    namespace: kube-system
spec:
    type: "nginx" 
    config:
        networkMode: container           # use container networkmode
        vip:
            enableLbSvc: true            # automatically creates an svc of type loadbalancer and treats the address assigned to the svc as the address of the alb
        loadbalancerName: alb-demo
        nodeSelector:
          alb-demo: "true"
        projects:
        - ALL_ALL
        replicas: 1
```
### gatewayapi
alb supports gatewayapi natively, just specify the gatewayclass as `exclusive-gateway` when creating the gateway.
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