## what it is
ALB (Alauda Load Balancer). a load balancer base on openresty which run in k8s. sometimes we use the same term alb2.
## file struct
./alb-nginx: stuff related with openresty,for more details see ./alb-nginx/README.md  
./Dockerfile: dockerfile of alb.  
./config ./controller ./driver ./hack ./ingress ./migrate ./modules ./pkg ./utils/ main.go go.mod go.sum : go code of  alb.  
./scripts: shell scripts for develop/debug alb,such as how to init alb env in kind.  
./template/nginx/lua: openresty lua code.  
./alauda: log rotate config file of alb.  
./Makefile: makefile of alb.
./chart: alb chart.  
./alb-config.toml: viper config file.  
./run.sh:   alb image entry point.  
./3rd-lua-module: "some lua module may not upload to opm or not the latest version" used in alb-nginx  
./test/e2e: env-test bootstrap e2e test  
./alb: use this package to split Init and Start logic from main package
## git repo 
https://gitlab-ce.alauda.cn/container-platform/alb2
## ci
http://confluence.alauda.cn/pages/viewpage.action?pageId=94878636
## doc
http://confluence.alauda.cn/label/cp/alb-doc
### labels of alb in confluence
alb-doc: all document that related to alb.
## maintainer
wucong congwu@alauda.io