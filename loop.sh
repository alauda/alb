#!/bin/bash

# showcase of a common alb develop workflow 
# tmux pane 0 shell # which run this script
# tmux pane 1 kubectl exec alb pod # edit deployment command to tail -f /dev/null first
# tmux pane 2 kubectl exec alb(nginx) pod # edit deployment command to tail -f /dev/null first

go-part() {
  local ALB_POD_NAME=$1
  make build
  md5sum ./bin/alb
  tmux send-keys -t 2 C-c
  kubectl cp $PWD/bin/alb cpaas-system/$ALB_POD_NAME:/alb/alb
  tmux send-keys -t 2 C-c 'md5sum /alb/alb' C-m
  sleep 1s
  tmux send-keys -t 2 C-c '/alb/alb 2>&1 | tee /alb.log' C-m
}

lua-part() {
  local ALB_POD_NAME=$1
  kubectl cp $PWD/template/nginx/lua  cpaas-system/$ALB_POD_NAME://alb/template/nginx
  tmux send-keys -t 2 C-c 'kill -s HUP 70' C-m
  sleep 2
  tmux send-keys -t 2 C-c 'curl -v http://127.0.0.1:80 -H "HOST: alb-dev.alauda.io"| grep "< " ' C-m
}

go-part $1
sleep 1s
lua-part $1
