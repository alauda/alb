#!/bin/bash
# replace alb in a running k8s,you'd better run patch-alb.sh first

./scripts/build.sh
tmux send-keys -t alb:0.0 C-c
alb=$(kubectl get pod -n cpaas-system|grep alb| awk '{print $1}'| tr -d '\n')
kubectl cp   ./alb ${alb}:/alb/alb  -c alb2 -n cpaas-system 
kubectl cp   ./alb-config.toml ${alb}:/alb/alb-config.toml  -c alb2 -n cpaas-system 
tmux send-keys -t alb:0.0 "md5sum ./alb/alb" Enter
sleep 2
# tmux send-keys -t alb:0.0 "export INTERVAL=10" Enter # just test ingress sync
# tmux send-keys -t alb:0.0 "export ALB_INCREMENT_SYNC=false" Enter 
# tmux send-keys -t alb:0.0 "export RESYNC_PERIOD=10" Enter 
# tmux send-keys -t alb:0.0 "export ALB_RELOAD_NGINX=false" Enter 
tmux send-keys -t alb:0.0 "./run.sh" Enter