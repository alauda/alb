# 创建ns
kubectl create ns timeout || true
kubectl label --overwrite ns timeout cpaas.io/project=timeout
# 部署应用
kubectl apply -f ./app.yaml -n timeout
# 部署alb
cat <<EOF | kubectl apply -f -
apiVersion: crd.alauda.io/v2
kind: ALB2
metadata:
    name: timeout-alb
    namespace: cpaas-system
spec:
    type: "nginx" 
    config:
        networkMode: container
        loadbalancerName: timeout-alb
        projects:
        - timeout
        replicas: 1
        vip:
            enableLbSvc: false
EOF
export ALB_IP=$(kubectl get pods -n cpaas-system -l service_name=alb2-timeout-alb -o jsonpath='{.items[*].status.podIP}')
echo $ALB_IP

# 在规则上设置timeout。
cat <<EOF | kubectl apply -n timeout -f  -
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: echo-resty
spec:
  rules:
  - http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: echo-resty
            port:
              number: 80
EOF

curl -X PUT -w "%{http_code}  %{time_total}" http://$ALB_IP/?sleep=2  -s -o /dev/null
应该在2s左右返回200 (因为我们设置sleep=2,而默认的timeout是120s)

kubectl annotate --overwrite ingress -n timeout echo-resty nginx.ingress.kubernetes.io/proxy-send-timeout=1s

kubectl annotate --overwrite ingress -n timeout echo-resty nginx.ingress.kubernetes.io/proxy-read-timeout=1s

curl -X PUT -w "%{http_code}  %{time_total}" http://$ALB_IP/?sleep=5  -s -o /dev/null
返回504 (大概5s,因为默认的重试策略设置的最大重试测试是5.所以会重试5次，所以是5s)

# 在ft （4层上）设置timeout
```bash
cat <<EOF | kubectl apply -f -
apiVersion: crd.alauda.io/v1
kind: Frontend
metadata:
  labels:
    alb2.cpaas.io/name: timeout-alb
  name: timeout-alb-00082
  namespace: cpaas-system
spec:
  backendProtocol: "http"
  certificate_name: ""
  port: 82
  protocol: tcp
  serviceGroup:
    services:
    - name: echo-resty
      namespace: timeout
      port: 80
      weight: 100
  config:
    timeout:
      proxy_connect_timeout_ms: 1000
      proxy_send_timeout_ms: 1000
      proxy_read_timeout_ms: 1000
EOF
```

curl -X PUT -w "%{http_code}  %{time_total}" http://$ALB_IP:82/?sleep=5  -s -o /dev/null
返回000 (大概1s)