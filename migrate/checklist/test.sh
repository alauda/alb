#!/bin/bash
set -o pipefail
source helper.sh
source ./custom/check-alb.sh
if [[ "$1" == "run-in-clusters" ]]; then
  # may read cluster from env
  eval "$2 global"
  eval "$2 p1"
else
  eval "$@"
fi
