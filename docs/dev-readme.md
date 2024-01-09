## what it is
ALB (Alauda Load Balancer). a load balancer base on openresty which run in k8s. sometimes we use the same term alb2.
## project struct
```plantuml
@startmindmap
* alb
** .build             (ci配置)
** cmd
*** operator          (operator的入口)
*** controller        (alb controller的入口)
*** utils             (一些工具binary)
**** sync_rbac        (更新rbac相应的yaml)
**** tweak_gen        (生成默认的configmap的yaml)
** config             (alb controller使用的config [应该迁移到pkg下])
** controller         (alb controller的代码 [应该迁移到pkg下])
** pkg
*** alb
*** apis              (自动生成的alb client的api)
*** client            (自动生成的alb client的client)
*** config            (operator和alb controller通用的配置解析相关的逻辑)
*** operator
** scripts            (alb开发/debug/测试相关的actions)
** template           (openresty的lua代码)
*** lua 
*** test              (lua的测试)
** test               (e2e测试)
** deploy             (部署所需要的chart)
*** chart             (alb-operator的chart)
*** resource          (生成chart,部署相关的一些资源)
** Dockerfile
** Dockerfile.local   (本地测试用,快速build一个alb的docker)
** run-alb.sh         (alb docker内的启动脚本)
@endmindmap
```
## image file struct
```plantuml
@startmindmap
* /alb/
** nginx/
*** /run     (volumed empty dir)
*** lua/
** ctl/
*** alb
** tools/
*** twwak_gen
** tweak/
* /etc/alb2/nginx/ (volume alb容器和nginx容器共享)
** nginx.conf
** policy.new
** nginx.pid
@endmindmap
```

## lint 
follow by ./scripts/alb-lint-actions.sh
## git repo 
https://gitlab-ce.alauda.cn/container-platform/alb2