#!/bin/zsh
# create a random ingress
cat <<EOF | kubectl apply -f -
apiVersion: networking.k8s.io/v1beta1
kind: Ingress
metadata:
  name: minimal-ingress-$RANDOM
  annotations:
    nginx.ingress.kubernetes.io/rewrite-target: /
spec:
  rules:
  - http:
      paths:
      - path: /testpath
        backend:
          serviceName: test
          servicePort: 80
      - path: /testpath1
        backend:
          serviceName: test
          servicePort: 80
EOF
