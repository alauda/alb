#!/bin/sh
if [ "$1" = "shell" ]
then
    docker run -v $PWD:/acp-alb-test -it registry.alauda.cn:60080/ops/golang:1.18-alpine3.15  sh
else
    docker run --network=host -e HTTP_PROXY=$HTTP_PROXY -e HTTPS_PROXY=$HTTPS_PROXY  -v $PWD:/acp-alb-test -it registry.alauda.cn:60080/ops/golang:1.18-alpine3.15  sh -c "cd /acp-alb-test ;/acp-alb-test/scripts/go-test.sh"
fi