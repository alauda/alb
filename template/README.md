## file structure
Dockerfile: the docker file of nginx which alb used, image registry: build-harbor.alauda.cn/3rdparty/alb-nginx.  
alb-test-runner.dockerfile: based on alb-nginx, env of test:nginx used in ci.  
directory t: test files of alb/template/nginx/lua.  
directory actions: scripts used in develop.  

## what template mean?
it should be renamed as openresty or resty-app ..
## QA
### what the different between this and [official-resty-image](https://github.com/openresty/docker-openresty/blob/59d43df176d2635f7fe429c58ffd0f307580614e/alpine/Dockerfile)
1. use alpine image
2. different build config
	1. nginx --with-debug
	2. install some lua module
		* lua-var-nginx-module            https://github.com/api7/lua-var-nginx-module/archive/v0.5.2
		* thibaultcha/lua-resty-mlcache   opm install thibaultcha/lua-resty-mlcache 
		* xiangnanscu/lua-resty-cookie    opm install xiangnanscu/lua-resty-cookie
		* lua-resty-balancer              https://github.com/openresty/lua-resty-balancer/archive/v0.04
	3. ignore luarocks
### 如何更新alb-nginx镜像
1. 更新alb-nginx/Dockerfile, alb-nginx流水线构建3rdparty/alb-nginx镜像
2. 更新alb-nginx/alb-test-runner.dockerfile中的alb-nginx镜像版本号, alb-test-runner-image流水线构建test-runner镜像
3. 更新本readme中的changelog, 修改chart/values.yaml中nginx镜像版本号
4. 更新alb2流水线使用的test-runner镜像版本, 构建alb镜像

### image changelog
#### alb-nginx
ci: https://build.alauda.cn/console-devops/workspace/acp/pipelines/all/alb-nginx 

alb-nginx:v3.6.0    
　　use openresty luajit

alb-nginx:v3.6.1

alb-nginx:20220118182511  
　　use ops/alpine as base image, upgrade openssl version in openresty to 1.1.1l

alb-nginx:20220317112016  
　　upgrade base-image to 3.15

alb-nginx:20220418150052  
　　update base-image 3.15

alb-nginx:20220424210109
　　upgrade openresty to 1.19.9.1  
　　upgrade openssl version in openresty to 1.1.1n

alb-nginx:v3.9-57-gb40a7de
    upgrade openssl to 1.1.1o
    update base-image to 3.16
    remove curl and related certs in /etc/ssl/certs

### alb-nginx-test
ci: https://build.alauda.cn/console-devops/workspace/acp/pipelines/all/alb-test-runner-image

alb-nginx-test:20220117144539  
　　add all needed to run test.

alb-nginx-test:20220317113027  
　　use alb-nginx:20220317112016.

alb-nginx-test:20220407172357
    use go1.18

alb-nginx-test:20220609230028
    use alb-nginx:v3.9-57-gb40a7de(openresty 1.19.9.1)

alb-nginx-test:20220711193217
    upgrade kubebuilder test-tools from 1.19.2 to 1.21.2
