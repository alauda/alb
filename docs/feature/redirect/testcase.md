ssl-redirect

# 在alb 上设置ssl-redirect
准备环境
```bash
kubectl create ns redirect || true
kubectl label ns --overwrite redirect cpaas.io/project=redirect

openssl req -x509 -newkey rsa:4096 -sha256 -days 3650 -nodes  -keyout ./key -out ./cert -subj  /CN="test.com"
openssl rsa -in ./key -traditional -out ./key-rsa
kubectl delete secret redirect-tls -n cpaas-system || true
kubectl create secret tls redirect-tls -n cpaas-system --key=./key-rsa --cert=./cert
# 部署应用
kubectl apply -f ./app.yaml -n redirect
```

部署alb
- 我们使用默认证书,ssl策略为both，让alb同时为http和https生成端口和规则。
- 通过在alb上设置ingressSSLRedirect: true，让alb的ingress-http端口在接收到http请求时自动重定向到https。

```bash
cat <<EOF | kubectl apply -f -
apiVersion: crd.alauda.io/v2
kind: ALB2
metadata:
    name: redirect-alb
    namespace: cpaas-system
spec:
    type: "nginx" 
    config:
        networkMode: container
        loadbalancerName: redirect-alb
        defaultSSLStrategy: "Both"
        defaultSSLCert: "cpaas-system/redirect-tls"
        ingressSSLRedirect: true
        projects:
        - redirect
        replicas: 1
        vip:
            enableLbSvc: false
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: redirect-ingress
  namespace: redirect
spec:
  rules:
    - host: a.com
      http:
        paths:
        - path: /
          pathType: Prefix
          backend:
            service:
              name: echo-resty
              port:
                number: 80
EOF
```

```bash
export ALB_IP=$(kubectl get pods -n cpaas-system -l service_name=alb2-redirect-alb -o jsonpath='{.items[*].status.podIP}')
echo $ALB_IP

# 308 redirect 命中了ingress规则。是http端口的规则，所以重定向到https端口。返回的header中 location 是 https://a.com/xxx
curl -k -v -H 'host: a.com' "http://$ALB_IP/xxx"

#308 redirect 没有命中ingress规则。是http端口，默认重定向到https端口(而不是返回404)。返回的header中 location 是 https://xx.com/xxx
curl -k -v -H 'host: xx.com' "http://$ALB_IP/xxx"

# 200 https规则。正常转发到后端
curl -k -v -H 'host: a.com' "https://$ALB_IP/xxx"

# 404 https规则。没有命中ingress规则，返回404
curl -k -v -H 'host: xx.com' "https://$ALB_IP/xxx"
```

# 在ingress上设置ssl-redirect。
不同于在alb上设置ingressSSLRedirect: true，在ingress上设置ssl-redirect，只会把命中规则的http请求重定向到https。

```bash
# alb同上但是去掉了ingressSSLRedirect: true
# 增加了一个ingress-b 来测试没有配置重定向的情况
cat <<EOF | kubectl apply -f -
apiVersion: crd.alauda.io/v2
kind: ALB2
metadata:
    name: redirect-alb
    namespace: cpaas-system
spec:
    type: "nginx" 
    config:
        networkMode: container
        loadbalancerName: redirect-alb
        defaultSSLStrategy: "Both"
        defaultSSLCert: "cpaas-system/redirect-tls"
        projects:
        - redirect
        replicas: 1
        vip:
            enableLbSvc: false
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: redirect-ingress
  namespace: redirect
  annotations:
    nginx.ingress.kubernetes.io/ssl-redirect: "true"
spec:
  rules:
    - host: a.com
      http:
        paths:
        - path: /
          pathType: Prefix
          backend:
            service:
              name: echo-resty
              port:
                number: 80
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: redirect-ingress-b
  namespace: redirect
spec:
  rules:
    - host: b.com
      http:
        paths:
        - path: /
          pathType: Prefix
          backend:
            service:
              name: echo-resty
              port:
                number: 80
EOF
```
```bash
# 308 redirect 命中规则。http端口。重定向到https. 返回的header中 location 是 https://a.com/xxx
curl -k -v -H 'host: a.com' "http://$ALB_IP/xxx"

# 200 没有命中规则。正常转发到后端
curl -k -v -H 'host: b.com' "http://$ALB_IP/xxx"

# 404 没有命中规则。返回404
curl -k -v -H 'host: xx.com' "http://$ALB_IP/xxx"

# 200 https规则。正常转发到后端
curl -k -v -H 'host: a.com' "https://$ALB_IP/xxx"
```