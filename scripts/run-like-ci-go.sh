#!/bin/sh
base="build-harbor.alauda.cn/ops/golang:1.18.3-alpine3.16"
if [ "$1" = "shell" ]; then
    docker run -v $PWD:/acp-alb-test -it "$base" sh
else
    proxy=""
    if [ -n "$USE_PROXY" ]; then
        proxy="--network=host -e HTTP_PROXY=$HTTP_PROXY -e HTTPS_PROXY=$HTTPS_PROXY "
    fi
    docker run $proxy -v $PWD:/acp-alb-test -it $base sh -c "cd /acp-alb-test ;/acp-alb-test/scripts/go-test.sh"
fi
