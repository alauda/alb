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
## ingress 
### ingress with other port
### support annotation
## container mode
## gatewayapi