#!/bin/bash

# showcase of a common alb envtest  develop workflow 
# tmux pane 0 # run make e2e-env-test
# tmux pane 1 # run this script
# tmux pane 2 # tail -f /tmp/alb-e2e-test/alb.log
# tmux pane 3 # run kubectl export KUBECONFIG=/tmp/env-test.kubecofnig

tmux send-keys -t 0 C-c ' make e2e-envtest' C-m
sleep 3s
tmux send-keys -t 2 C-c ' tail -f /tmp/alb-e2e-test/alb.log' C-m
