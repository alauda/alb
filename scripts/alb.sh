#!/bin/sh
tmux kill-session -t alb
tmux new-session -d -s alb
tmux rename-window 'alb'
tmux splitw -h
tmux select-layout tiled
tmux splitw -h
tmux splitw -h
tmux select-layout tiled
tmux select-layout tiled
tmux selectp -t 0
tmux send-keys 'cd "./"' 'C-m'
tmux select-pane -T 'alb-nginx-log'
tmux send-keys './scripts/alb-nginx-log.sh' 'C-m'
tmux selectp -t 1
tmux send-keys 'cd "./"' 'C-m'
tmux select-pane -T 'alb-port-forward'
tmux send-keys './scripts/alb-port-forward.sh' 'C-m'
tmux selectp -t 2
tmux send-keys 'cd "./"' 'C-m'
tmux select-pane -T 'alb-run'
tmux send-keys './scripts/into-alb.sh' 'C-m'
tmux send-keys './run.sh' 'C-m'
tmux selectp -t 3
tmux send-keys 'cd "./ - echo "ok""' 'C-m'
tmux select-pane -T 'shell'
tmux -2 attach-session -d