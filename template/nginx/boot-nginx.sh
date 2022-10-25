#!/bin/sh

_term() {
  ngx=$(cat /etc/alb2/nginx/nginx.pid)
  if [ -z "$ngx" ]; then
    echo "nginx not start up,just exit"
    exit 0
  fi
  echo "Caught SIGTERM signal! term $ngx"
  local max_term_seconds="$MAX_TERM_SECONDS"
  if [ -z "$max_term_seconds" ]; then
    max_term_seconds="30"
  fi
  echo "wait $max_term_seconds"
  sleep $max_term_seconds
  echo "send term to nginx $ngx"
  kill -TERM "$ngx" 2>/dev/null
  echo "send term to nginx $ngx over"
  exit 0
}

trap _term SIGTERM
trap _term SIGQUIT

sudo -E sh -c 'ulimit -n 100000 && gosu nonroot:nonroot /alb/nginx/run-nginx.sh' &
child=$!
wait "$child"
