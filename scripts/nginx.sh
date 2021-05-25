#!/usr/bin/env bash
# Rebuild alb-nginx and remove old image
# Kill old alb-nginx
# Run alb-nginx locally with int jakiro and region int_azure_k8s

cd `dirname $0`; cd ..
docker build -t alb-nginx -f Dockerfile.nginx .

docker rm -f alb-nginx
docker rmi $(docker images | grep none|awk '{print $3}')

docker run -d -e NAMESPACE=admin \
              -e KUBERNETES_SERVER=https://139.219.184.215:6443 \
              -e KUBERNETES_SERVER=https://139.219.184.215:6443 \
              -e KUBERNETES_BEARERTOKEN=129a85.b94140948e11461c \
              -e JAKIRO_ENDPOINT=http://42.159.234.24:20081 \
              -e NAME=haproxy-139-219-184-215 \
              -e REGION_NAME=int_azure_k8s \
              -e LB_TYPE=nginx \
              -e SCHEDULER=kubernetes \
              -e TOKEN=3cd2761384344ca03d4987b253e8ff169a9e7585 \
              --name=alb-nginx \
              -p 1936:1936 \
              -p 80:80 \
              alb-nginx:latest

docker exec -it alb-nginx sh
