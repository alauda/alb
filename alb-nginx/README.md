## file structure
Dockerfile: the docker file of nginx which alb used,image build-harbor.alauda.cn/3rdparty/alb-nginx.  
openresty-testframe.dockerfile: base on alb-nginx,env of test:nginx used in ci.  
t: test file of alb/template/nginx/lua.  
actions: scripts used in develop.  
## QA
### what the different between this and [official-resty-image](https://github.com/openresty/docker-openresty/blob/1.19.3.2-1/bionic/Dockerfile)
1. use alpine image
2. use custom luajit version
3. different build config
	1. nginx --with-debug
	2. install some lua module
		* lua-var-nginx-module
		* thibaultcha/lua-resty-mlcache
		* xiangnanscu/lua-resty-cookie
		* lua-resty-balancer
	3. ingore luarocks
### why use custom luajit version instead of built in
1. to fix http://jira.alauda.cn/browse/ACP-5137 we need luajit which contains commit 787736990ac3b7d5ceaba2697c7d0f58f77bb782.
2. i dont know what the commit id 787736990ac3b7d5ceaba2697c7d0f58f77bb782 mean (commit 67dbec82f4f05a416a78a560a726553beaa7a223 behind this seems more meaningful).
3. to keep the compatibility we need to make sure all of luajit used here contains this commit.
4. different version of luajit will cause huge different when benchmark. watch it out.
5. the current header of [luajit2](https://github.com/openresty/luajit2)(today is 20210623) is 886d5f895b8ae19def724677376322b1f7ae2d4a.
6. this step should be removed when openresty built in luajit update.
### 如何更新alb-nginx镜像
1. 修改alb-nginx/Dockerfile alb-nginx流水线构建镜像
2. 更新alb-nginx/alb-test-runner.dockerfile中alb-nginx镜像版本号,alb-test-runner-image流水线构建test-runner镜像
3. 更新本readme中的changelog,修改chart/values.yaml中nginx镜像版本号
5. 更新alb2流水线使用的test-runner镜像版本,构建alb镜像.
### image changelog
#### alb-nginx
ci: https://build.alauda.cn/console-devops/workspace/acp/pipelines/all/alb-nginx 

alb-nginx:v3.6.0  
	use openresty luajit

alb-nginx:v3.6.1
alb-nginx:20220118182511
	use ops/alpine as base image,upgrade openssl versin in openresty to 1.1.1l
alb-nginx:20220317112016
	upgrade base-image to 3.15
alb-nginx:20220418150052
	update base-image 3.15


### alb-nginx-test
ci: https://build.alauda.cn/console-devops/workspace/acp/pipelines/all/alb-test-runner-image
alb-nginx-test:20220117144539
	add all needed to run test.
alb-nginx-test:20220317113027
	use alb-nginx:20220317112016.