# 部署alb。在ft上设置4层
```bash
kubectl create ns keepalive || true
kubectl label ns keepalive cpaas.io/project=keepalive
kubectl apply -n keepalive -f ./app.yaml

cat <<EOF | kubectl apply -f -
apiVersion: crd.alauda.io/v2
kind: ALB2
metadata:
    name: keepalive-alb
    namespace: cpaas-system
spec:
    type: "nginx"
    config:
        networkMode: container
        loadbalancerName: keepalive-alb
        replicas: 1
        projects:
        - keepalive
        vip:
            enableLbSvc: false
---
apiVersion: crd.alauda.io/v1
kind: Frontend
metadata:
  labels:
    alb2.cpaas.io/name: keepalive-alb
  name: keepalive-alb-00081
  namespace: cpaas-system
spec:
  backendProtocol: "tcp"
  certificate_name: ""
  port: 81
  protocol: tcp
  serviceGroup:
    services:
    - name: echo-resty
      namespace: keepalive
      port: 80
      weight: 100
  config:
    keepalive:
        tcp:
          idle: 60m
          interval: 30s
          count: 3
---
apiVersion: crd.alauda.io/v1
kind: Frontend
metadata:
  labels:
    alb2.cpaas.io/name: keepalive-alb
  name: keepalive-alb-00082
  namespace: cpaas-system
spec:
  backendProtocol: "http"
  certificate_name: ""
  port: 82
  protocol: http
  serviceGroup:
    services:
    - name: echo-resty
      namespace: keepalive
      port: 80
      weight: 100
  config:
    keepalive:
        http:
         timeout: 120m
         requests: 12000
EOF
```


```bash
export ALB_IP=$(kubectl get pods -n cpaas-system -l service_name=alb2-keepalive-alb -o jsonpath='{.items[*].status.podIP}')
echo $ALB_IP
#  81和82端口的转发正常
curl "http://$ALB_IP:82"
curl "http://$ALB_IP:81"
export ALB_POD=$(kubectl get pods -n cpaas-system -l service_name=alb2-keepalive-alb -o jsonpath='{.items[*].metadata.name}')
echo $ALB_POD

# 检查nginx配置文件中有4层keepalive配置 
# 4层关键字: so_keepalive=60m:30s:3;
# 7层关键字: keepalive_timeout 120m; keepalive_requests 12000;
kubectl exec -it -n cpaas-system $ALB_POD -c nginx -- cat /etc/alb2/nginx/nginx.conf | grep keepalive
```