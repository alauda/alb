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
		* thibaultcha/lua-resty-mlcache   opm install thibaultcha/lua-resty-mlcache    2.5.0
        * Kong/lua-resty-worker-events    2.0.0 clone in repo
		* xiangnanscu/lua-resty-cookie    opm install xiangnanscu/lua-resty-cookie     0.0.1 https://github.com/xiangnanscu/lua-resty-cookie/commit/948b77f8a5f2c9f1cdc28b4c6cd5a60d64b4fab7
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

# cache
rule_cache cert_cache and backend_cache
缓存层级为
l1 worker lrucache
l2 shared dict
l3 callback
0. 按照缓存层级获取数据 1->2->3
1. 只有在master线程中读配置文件
2. 读取配置文件之后，会直接更新l2
3. 通过mlcache的delete 删除掉过期的配置
3.1 当删除时，会调用ipc的boardcast，最终将所有worker的l1 cache的配置删除 (mlcache中注册了ipc，用lua-resty-worker-events做的所有worker的同步)
4. 当获取数据时，会因为配置被删除了，而去从l2上获取信息 或者调用callback？当调用完毕之后会将结果缓存在l1上

当每次请求到达时 调用get_upstream 会
1. cache.update 触发事件机制流程，如果其他worker有delete事件，这次update会导致自己的l1的cache中的过期的key被删掉 
2. cache.get 获取对应端口的所有规则 并反序列化成lua的table 这是会更新l1
3. 遍历规则并判断
cache结构
l2 http_policy 
    port           each port
    all_policies   all policy
l2 cert_cache 
    domain         each domain
    certficate_map all cert
l2 http_backend_cache
    backend_group all backend
master每隔1s
1.fetch policy会更新l2的 rule_cache cert_cache 和 http_backend_cache 
每个worker每隔1s
从l2中获取http_backend_cache，同步到自己的balancers变量中