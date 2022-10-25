#!/bin/bash
CFD="$(cd "$(dirname "${BASH_SOURCE[0]}")" &>/dev/null && pwd)"
ALB=$CFD/../../
source $CFD/common.sh

test-nginx-exec $1
