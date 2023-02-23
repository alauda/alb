## what it is
ALB (Alauda Load Balancer). a load balancer base on openresty which run in k8s. sometimes we use the same term alb2.
## project struct
```plantuml
@startmindmap
* alb
** .build
** cmd
*** operator
*** controller
*** migrate(迁移工具(legacy))
*** tools
** pkg
*** operator
*** config
*** alb
**** ctl
***** ingress
***** gateway
**** nginx
*** apis
*** client
*** util
** nginx(openresty-app)
*** lua 
*** test
** deploy (部署所需要的bundle/chart)
*** resource
**** crds
*** chart
**** alb
***** Chart.yaml
**** alb-operator
***** Chart.yaml
*** operator(operator csv/bundle的配置)
** scripts
*** hack
** test
** Dockerfile
** entry
*** run-alb.sh
*** boot-nginx.sh
*** run-nginx.sh
@endmindmap
```
## image file struct
```plantuml
@startmindmap
* /alb/
** nginx/
*** lua/
** ctl/
*** alb
** tools/
*** twwak_gen
** tweak/
* /etc/alb2/nginx/ (alb容器和nginx容器共享)
** nginx.conf
** policy.new
** nginx.pid
@endmindmap
```

## lint 
follow by ./scripts/alb-lint-actions.sh
## git repo 
https://gitlab-ce.alauda.cn/container-platform/alb2
## ci
http://confluence.alauda.cn/pages/viewpage.action?pageId=94878636
## doc
http://confluence.alauda.cn/label/cp/alb-doc
### labels of alb in confluence
alb-doc: all document that related to alb.