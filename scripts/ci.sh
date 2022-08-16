#!/bin/bash

pwd
set -e
export ALB=$PWD
export ALB_ACTIONS_ROOT=$ALB/scripts
echo $ALB
source ./scripts/alb-dev-actions.sh
alb-test-all-in-ci