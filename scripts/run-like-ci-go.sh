#!/bin/sh
set -x
proxy=""
if [ -n "$USE_PROXY" ]; then
  proxy="--network=host -e http_proxy=$HTTP_PROXY -e https_proxy=$HTTPS_PROXY "
fi
base=${1-$(cat ./Dockerfile | grep GO_BUILD_BASE | awk -F = '{print $2}')}
echo "$base"
docker run $proxy -v $PWD:/acp-alb-test -e ALB_ONLINE=$ALB_ONLINE -t $base sh -c "cd /acp-alb-test ;/acp-alb-test/scripts/go-test.sh"
