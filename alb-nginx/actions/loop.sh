# showcase of a common alb-nginx develop workflow
# tmux pane 0 test-nginx-exec.sh
# tmux pane 1 shell # which run this script
# tmux pane 2 docker-exec # cat error.log
# tmux pane 2 docker-exec # cat access.log
echo "loop start"
tmux send-keys -t 0 C-c '  prove -I / -I /test-nginx/lib/ -r t/ping.t' C-m
sleep 1s
tmux send-keys -t 2 C-c '   cat /t/servroot/logs/error.log' C-m
tmux send-keys -t 3 C-c '   cat /t/servroot/logs/access.log' C-m
