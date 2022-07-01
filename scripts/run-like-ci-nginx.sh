#!/bin/sh
# test current nginx
# we should build a nginx image first then use this image to test.
tag=$(yq '.global.images.nginx.tag' ./chart/values.yaml)
image=build-harbor.alauda.cn/3rdparty/alb-nginx:$tag

docker run --network=host -e HTTP_PROXY=$HTTP_PROXY -e HTTPS_PROXY=$HTTPS_PROXY  -v $PWD:/acp-alb-test -it $image  sh -c 'cd /acp-alb-test ;/acp-alb-test/scripts/nginx-test.sh'