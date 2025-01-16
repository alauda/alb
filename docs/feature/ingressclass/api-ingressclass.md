# 1. 概述
如何只通过ingressclass来提供足够的信息，来显示ingressclass的下拉列表
## 1.1 编写目的
1. 如何获取ingresclass，以及如何根据ingressclass上的annotation和label区分出这个ingressclass
2. 显示ingressclass 下拉列表时的注意事项
## 1.2 项目背景
1. 在公有云上，可能会存在多个ingress controller，用户可能希望使用这些ingress controller而不是alb
2. 通过让用户创建ingress时显示设置ingressclass，来达到选择ingress controller的目的
## 1.3 术语定义
序号	术语	说明	备注
## 1.4 参考资料
序号	参考资料	备注
# 2 功能清单
汇总产品所有功能清单列表
序号	功能名称	功能描述	备注
# 3 详细功能设计
## 3.1 模块一
### 3.1.1 模块功能描述
### 3.1.6 API 设计
总述API要实现的基本目标
#### 3.1.6.1  GET  kubernetes/{cluster}/apis/networking.k8s.io/v1/ingressclasses
##### 3.1.6.1.1 说明 ingressclass下拉列表处理步骤
1. 获取所有ingressclass
2. 根据ingressclass的annotation 区分出alauda alb，和公有云默认的ingress class和用户部署的其他ingressclass
    区分方式为
        1. 带有alb.cpaas.io/managed-by: alb-operator 的label的是alauda alb
            1.1 根据alb.cpaas.io/project，的key来获取项目 value格式为逗号分割的字符串，每个是一个项目 
3. 过滤掉非当前项目的alb的ingressclass
    1. 从ns的label中，用cpaas.io/project拿到这个ns的项目
    2. 有了ns的项目，和alb的项目，就可以做过滤了，注意alb的项目上可能会有ALL_ALL
        这种是所有项目，可以匹配所有的项目
4. 如果ingressclass有default的，默认选中它
5. 如果是公有云，并且没有对应的ingressclass。在对应公有云上，下拉列表中加上对应的ingressclass
    cce(huawei): cce
    gke(google): gce
 并且标识为云
6. ake(auzure)上如果有名字是webapprouting.kubernetes.azure.com的ingressclass，那么标识为云
   eks(aws)上如果有名字是alb的ingressclass，那么标识为云
7. 其他标识为其他

```yaml
# ingressclass demo
apiVersion: networking.k8s.io/v1
kind: IngressClass
metadata:
  annotations:
    alb.cpaas.io/project: ALL_ALL,p1,p2
    ingressclass.kubernetes.io/is-default-class: "false"
  creationTimestamp: "2023-08-23T02:43:16Z"
  generation: 1
  labels:
    alb.cpaas.io/alb2-operator: cpaas-system_alauda-alb
    alb.cpaas.io/alb2-operator-albname: alauda-alb
    alb.cpaas.io/alb2-operator-albns: cpaas-system
    alb.cpaas.io/managed-by: alb-operator
    alb.cpaas.io/version: v3.14.0-beta.11.g9bde2d00
  name: alauda-alb
  resourceVersion: "477107"
  uid: 0b77efab-ccd7-477c-a478-31b3edf814c0
spec:
  controller: cpaas.io/alb2
```


###### 关于端口模式alb的ingressclass的项目的annotation的额外说明
端口模式的项目，实际上是
`portProjects: '[{"port":"111-2233","projects":["ALL_ALL"]},{"port":"12222-33445","projects":["cong"]}]'`
但是对于端口模式alb处理ingress来讲，只有
1. ingress http https端口被用户创建
2. ingress http https端口的项目是有用户ns的项目的权限的

所以在端口项目的alb的ingressclass中，项目，是ingress http https端口的项目


##### 3.1.6.1.2 请求参数
##### 3.1.6.1.3 返回参数
##### 3.1.6.1.4 示例（可选）
##### 3.1.6.2.5 变更影响说明
#### 3.1.6.2  POST  kubernetes/{cluster}/apis/networking.k8s.io/v1/namespaces/{NS}/ingresses
##### 3.1.6.2.1 说明 创建ingres时指定ingressclass
##### 3.1.6.1.4 示例
注意
1. 如果是在gke上，并且使用了gce的ingressclass。 那么只能设置annotation
    https://cloud.google.com/kubernetes-engine/docs/concepts/ingress?hl=zh-cn
2. 其他情况只能设置ingressclass
```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  annotations:
    kubernetes.io/ingress.class: "{ingcls}" # 
  name: xxxxxxx
  namespace: cong
spec:
  rules:
  - host: ddddd
    http:
      paths:
      - backend:
          service:
            name: xx-sq7bn
            port:
              number: 1936
        path: /ss
        pathType: Prefix
```
```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: xxxxxxx
  namespace: cong
spec:
  ingressClassName: {ingcls}
  rules:
  - host: ddddd
    http:
      paths:
      - backend:
          service:
            name: xx-sq7bn
            port:
              number: 1936
        path: /ss
        pathType: Prefix
```

