#!/bin/bash
# replace the entry-point of alb pod to tail,since that could replace and restart alb
lbName="alb-wc"
kubectl patch deployment -n cpaas-system $lbName --patch '
spec:
  template:
    spec:
      containers:
      - name: alb2
        command: ["tail","-f","/dev/null"]'

# kubectl patch deployment \
#   alb-wc \
#   --namespace cpaas-system \
#   --type='json' \
#   -p='[
#     {"op": "replace", "path": "/spec/template/spec/containers/0/args", "value": ["-f"]},
#     {"op": "replace", "path": "/spec/template/spec/containers/0/command", "value": ["tail"]}]'