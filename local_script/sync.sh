#!/bin/bash

get_panel_index() {
    pane=$(	tmux list-panes -s -F "#{session_name}:#{window_name}:#{window_index}:#{pane_title}:#{pane_index}" |grep alb2-shell-run)
    pane_index=$(echo $pane |cut -d ':' -f 5)
    echo $pane_index
}

POD=$(kubectl get po -n cpaas-system |grep alb |cut -d " " -f 1| tr -d '\n')

send_shell() {
    cmd=$1
    run_panel=alb2:alb2.$(get_panel_index alb2-shell-run)
    shell_panel=alb2:alb2.$(get_panel_index alb2-shell-shell)

    tmux send-keys -t $run_panel "$cmd" Enter
}


build_replace_and_run() {
    ../local_script/build.sh
    # send_shell "kubectl exec -it -n cpaas-system $pod /bin/sh"
    # run_in_shell "kubectl exec -it -n cpaas-system $pod /bin/sh"
    send_shell "C-c"
    kubectl cp  ./alb cpaas-system/$pod:/alb/alb
    send_shell "md5sum ./alb/alb"
    send_shell "./alb/alb"
}
