#!/bin/bash
env
set -o pipefail
source helper.sh
source ./custom/check-alb.sh
echo "$@"
BACKUP_DIR="./"
if [[ -n "$2" ]]; then
  prdb_version="$2"
fi
if [[ -n "$3" ]]; then
  target_version="$3"
fi
eval "$1"
