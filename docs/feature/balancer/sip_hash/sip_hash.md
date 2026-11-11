sip-hash(source-ip-hash)
when alb pick a backend server for a request, it will could use sip-hash(source-ip-hash) algorithm to pick a backend server. which means the same source ip will be picked the same backend server.
## requirements
deploy a alb first.
```bash
cat <<EOF | kubectl apply -f -
apiVersion: crd.alauda.io/v2
kind: ALB2
metadata:
    name: sip-hash-alb
    namespace: cpaas-system
spec:
    type: "nginx" 
    config:
        networkMode: container
        loadbalancerName: sip-hash-alb
        projects:
        - ALL_ALL
        replicas: 1
        vip:
            enableLbSvc: false
EOF

kubectl apply -f ./common-app.yaml -n default 
kubectl scale  deployments.apps -n default echo-resty --replicas 3

export ALB_IP=$(kubectl get pods -n cpaas-system -l service_name=alb2-sip-hash-alb -o jsonpath='{.items[*].status.podIP}')
echo $ALB_IP
```
## l4
### how to config
#### via ft
```bash
cat <<EOF | kubectl apply -f -
apiVersion: crd.alauda.io/v1
kind: Frontend
metadata:
  labels:
    alb2.cpaas.io/name: sip-hash-alb
  name: sip-hash-alb-11180
  namespace: cpaas-system
spec:
  port: 11180
  protocol: tcp
  serviceGroup:
    services:
    - name: echo-resty
      namespace: default
      port: 80
      weight: 100
    session_affinity_policy: "sip-hash"
EOF
```
```bash
curl $ALB_IP:11180 # you will notice that is always the same backend server.
```

## l7
### how to config
if use request has x-forwarded-for header, the first x-forwarded-for will be treated as the source ip.

#### via ingress
```bash
cat <<EOF | kubectl apply -n default -f  -
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: echo-resty-sip-hash
spec:
  ingressClassName: sip-hash-alb
  rules:
  - http:
      paths:
      - path: /1
        pathType: Prefix
        backend:
          service:
            name: echo-resty
            port:
              number: 80
EOF
```
```bash
curl -H 'x-forwarded-for: 127.0.0.5' http://$ALB_IP:80/1 # change the x-forwarded-for header to see the different backend server.
```
#### via rule
```bash
cat <<EOF | kubectl apply  -f  -
apiVersion: crd.alauda.io/v1
kind: Rule
metadata:
  labels:
    alb2.cpaas.io/frontend: sip-hash-alb-00080
    alb2.cpaas.io/name: sip-hash-alb
  name: sip-hash-alb-00080-demo
  namespace: cpaas-system
spec:
  dslx:
  - type: URL
    values:
    - - STARTS_WITH
      - /2
  priority: 5
  serviceGroup:
    session_affinity_policy: "sip-hash"
    services:
    - name: echo-resty
      namespace: default
      port: 80
      weight: 100
EOF
```

```bash
curl -H 'x-forwarded-for: 127.0.0.5' http://$ALB_IP:80/2 # change the x-forwarded-for header to see the different backend server.
```