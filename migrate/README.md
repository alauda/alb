alb1->alb2升级脚本使用说明

需要在1.15环境测试，alb1版本使用（http://jira.alaudatech.com/browse/DEV-13122） 
先不安装alb2. 
直接去集群内 
1. 安装alb2的crd，并新建一个alauda-system的k8s namespace用来存放alb2的自定义资源 
2. docker run -it index.alauda.cn/alaudaorg/alb2:v2.1-7-gfe11214 sh 
3. export KUBERNETES_BEARERTOKEN=dbf41d.ae570f0cc2527039 ; export KUBERNETES_SERVER=https://140.143.218.82:6443 
4. export NAME=haproxy-140-143-49-47 
5. cd alb 
6. ./migrate -dry-run=false 
7. 安装alb2，注意定点部署的时候不要和集群内alb1在一台机器（参考http://confluence.alaudatech.com/pages/viewpage.action?pageId=29428188） 
8. 测试以前访问的负载均衡地址在alb2上能否访问 
（例如以前 curl -H 'Host: jakiro-test.int-tencent.haproxy-140-143-49-47-alaudaorg.myalauda.cn' 140.143.49.47） 
现在就把后面的ip换成alb2主机的ip