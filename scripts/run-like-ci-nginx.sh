#!/bin/sh
# test current nginx
# we should build a nginx image first then use this image to test.
tag=$(yq '.global.images.nginx.tag' ./deploy/chart/alb/values.yaml)
image=build-harbor.alauda.cn/acp/alb2:$tag
if [ -n "$1" ]; then
  image="$1"
fi
# image=alb-nginx:test

platform=${MATRIX_PLATFORM:-linux/amd64}
echo "platform $platform"
docker run --user root --network=host --platform $platform -v $PWD:/acp-alb-test -t $image sh -c 'cd /acp-alb-test ;/acp-alb-test/scripts/nginx-test.sh'
