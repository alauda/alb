#!/bin/sh
# test current nginx
# we should build a nginx image first then use this image to test.
tag=$(yq '.global.images.nginx.tag' ./chart/values.yaml)
if [ ! "$1" = "" ]; then
  tag="$1"
fi
image=build-harbor.alauda.cn/acp/alb-nginx:$tag

docker run --network=host -e http_proxy=$HTTP_PROXY -e https_proxy=$HTTPS_PROXY -v $PWD:/acp-alb-test -it $image sudo -E sh -c 'cd /acp-alb-test ;/acp-alb-test/scripts/nginx-test.sh'
