#!/bin/bash
env
set -o pipefail
# set -x
if [[ -n "$2" ]]; then
  export prdb_version="$2"
fi
if [[ -n "$3" ]]; then
  export target_version="$3"
fi
export backup_dir="./"
source helper.sh
source ./custom/check-alb.sh
eval "$1"
