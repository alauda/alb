#!/bin/bash
# shellcheck disable=SC2046,SC2086,SC1091,SC2005

function alb-log-show-error() {
  #
  local f=$1
  echo "skip access log"
  rg -v '"time"' $f | rg -v 'closed keepalive' | rg -v 'timed out'
  echo 'show retry log'
  jq -r '"\(.time) \(.upstream_addr) \(.upstream_status)"' $1 | sort | uniq -c | rg ','

}
