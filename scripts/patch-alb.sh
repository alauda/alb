#!/bin/bash
# replace the entry-point of alb pod to tail,since that could replace and restart alb
kubectl patch deployment -n cpaas-system alb2-wc --patch '
spec:
  template:
    spec:
      containers:
      - name: alb2
        command: ["tail","-f","/dev/null"]