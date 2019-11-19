#!/usr/bin/env bash
docker rm nginx-dev
docker run -it -p 8080:8080 -v /Users/oilbeater/workspace/alb-nginx/template:/alb/template -v /Users/oilbeater/workspace/alb-nginx/conf:/alb/conf --name=nginx-dev index.alauda.cn/alaudaorg/nginx:v0.0.1 sh