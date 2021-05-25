#!/bin/zsh
# update ingress to random path
set -e

random_path=$(openssl rand -hex 12)
echo $random_path
kubectl get ing/alb-wc -n cpaas-system -o json|jq ".spec.rules[0].http.paths[0].path=\"/${random_path}\""| kubectl apply -f-
kubectl get ing/alb-wc -n cpaas-system -o json| jq ".spec.rules[0].http.paths[0].path,.metadata.resourceVersion"