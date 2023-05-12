#!/bin/bash
CFD="$(cd "$(dirname "${BASH_SOURCE[0]}")" &>/dev/null && pwd)"
ALB=$(realpath $CFD/../../)
source $CFD/alb-nginx.sh

test-nginx-in-ci
