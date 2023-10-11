#!/bin/bash
set -o pipefail
source helper.sh
source ./custom/helper.sh
source ./custom/check-alb-project.sh
source ./custom/check-alb-ingressport.sh
source ./custom/check-alb-resource.sh
echo "$@"
if [[ -n "$2" ]]; then
  prdb_version="$2"
fi
if [[ -n "$3" ]]; then
  target_version="$3"
fi
eval "$1"
