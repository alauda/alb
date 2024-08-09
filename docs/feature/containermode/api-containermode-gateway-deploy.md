# 1. 概述
容器网络模式gateway的部署相关api
## 1.1 编写目的
概述编写此文档的目的

## 1.2 项目背景
整体介绍项目背景，目前状态以及缘由
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
#### 3.1.6.1  GET  kubernetes/{cluster}/apis/gateway.networking.k8s.io/v1beta1/gatewayclasses
##### 3.1.6.1.1 说明 获取独享型网络类列表
我们默认部署的独享型gatewayclass上会带有
```yaml
gatewayclass.cpaas.io/deploy: cpaas.io
gatewayclass.cpaas.io/type: standalone
```
这两个label，要用这两个label来过滤,默认部署的独享型gatewayclass的name是"exclusive-gateway"
如果是共享型的,label是
```
gatewayclass.cpaas.io/deploy: cpaas.io
gatewayclass.cpaas.io/type: shared
```
共享型gateway的访问地址从alb上获取
1. alb.spec.address 是用户手动设置的地址，格式为逗号分割的字符串
2. alb.status.detail.address, 是开启lbsvc后分配的地址。格式为
```
apiVersion: crd.alauda.io/v2beta1
kind: ALB2
metadata:
  labels:
    gatewayclass.cpaas.io/deploy: cpaas.io
    gatewayclass.cpaas.io/type: shared
  name: global-alb2
  namespace: cpaas-system
spec:
  address: 127.0.0.1,127.0.0.2
  config:
    loadbalancerName: global-alb2
    replicas: 1
  type: nginx
status:
  detail:
    address:
      ipv4:
        - 128.0.0.1
      ipv6:
        - fe80::6442:d6ae:ffd8:6f07
      msg: ""
      ok: true
    alb: {}
    deploy:
      probeTimeStr: "2023-06-30T02:28:43Z"
      reason: workload ready
      state: Running
    version:
      imagePatch: not patch
      version: v3.14.1
  probeTime: 1688092123
  probeTimeStr: "2023-06-30T02:28:43Z"
  reason: ""
  state: Running
```



##### 3.1.6.1.2 请求参数
##### 3.1.6.1.3 返回参数
##### 3.1.6.1.4 示例（可选）
##### 3.1.6.2.5 变更影响说明

#### 3.1.6.2 创建容器网络gateway
##### 3.1.6.2.1 说明
创建容器网络gateway需要先创建alb，在创建gateway
###### 创建alb  POST  /kubernetes/{cluster}/apis/crd.alauda.io/v2beta1/namespaces/{namespace}/alaudaloadbalancer2 
```yaml
apiVersion: crd.alauda.io/v2beta1
kind: ALB2
metadata:
  name: g1-12334 
  namespace: a1
spec:
  config:
    enableAlb: false
    resources:
      limits:
        cpu: 200m
        memory: 256Mi
      requests:
        cpu: 200m
        memory: 256Mi
    vip:
        enableLbSvc: true
        lbSvcAnnotations:
           a: b
    gateway:
        mode: standalone
        name: g1
  type: nginx
```
###### 创建gateway  /kubernetes/{cluster}/apis/gateway.networking.k8s.io/v1beta1/namespaces/{NS}/gateways
```yaml
apiVersion: gateway.networking.k8s.io/v1beta1
kind: Gateway
metadata:
    name: g1
    namespace: a1
    labels:
      alb.cpaas.io/alb-ref: g1-12334
spec:
    gatewayClassName:  exclusive-gateway
    listeners:
    - name: http
      port: 8234
      protocol: HTTP
      hostname: a.com
      allowedRoutes:
        namespaces:
          from: All
```
###### 注意
1. 要先创建alb再创建gateway
2. alb的namespace和gateway是一样的,alb的name是gateway name+5位随机字符,必须在alb的config.gateway.name中指定gateway的名字
3. 当gateway.mode为standalone-gateway时，networkmode默认为container
4. 在gateway上通过alb.cpaas.io/alb-ref 指出这个gateway所使用的alb

#### 3.1.6.3 更新容器网络模式gateway
##### 3.1.6.3.1 说明
如果是更新gateway相关的配置比如listener之类的，只更新gateway即可。如果是更新alb相关的，比如部署资源，lbsvcAnnotation只更新alb即可
* 更新alb PUT /kubernetes/{cluster}/apis/crd.alauda.io/v2beta1/namespaces/{namespace}/alaudaloadbalancer2/{NAME}
* 更新gateway PUT /kubernetes/{cluster}/apis/gateway.networking.k8s.io/v1beta1/namespaces/{NS}/gateways/{NAME}
#### 3.1.6.4 删除gateway DELETE  /kubernetes/{cluster}/apis/gateway.networking.k8s.io/v1beta1/namespaces/{NS}/gateways/{NAME}
##### 3.1.6.4.1 说明
只用删除gateway即可，后端会自动删除相应的alb.
### 3.1.7 模块部署设计
模块部署过程以及部署依赖等

### 3.1.8 权限设计
模块设计实现中涉及到的权限部分说明。比如新增crd的权限设计，api调用者的权限约束说明等。

### 3.1.9 非功能设计
#### 3.1.9.1 性能设计
该设计对于性能有何影响，或为了解决性能问题，应该采取何种设计

#### 3.1.9.2 兼容性设计
需要支持《ACP 兼容性需求基线》里规定的各种环境

#### 3.1.9.3 安全设计
必须满足《灵雀云公司产品通用安全基线(安全红线)》，当前设计方案有哪些可能带来安全问题，如何解决。


# 4. 部署升级方案
## 4.1. 部署方案
## 4.2. 升级方案
## 4.3. 单点和高可用方案


# 5. 可用性设计
## 5.1. 数据备份和恢复方案
## 5.2. 容灾设计方案
# 6. 运维能力设计
## 6.1. 监控方案