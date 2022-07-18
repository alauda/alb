#!/bin/sh
pwd
sed -i 's/dl-cdn.alpinelinux.org/mirrors.ustc.edu.cn/g' /etc/apk/repositories
apk add bash
bash -c 'ALB=$PWD;source ./scripts/alb-dev-actions.sh;alb-test-all-in-ci-nginx'