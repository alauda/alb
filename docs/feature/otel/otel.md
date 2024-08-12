# 配置OpenTelemetry
## 概述
> OpenTelemetry（OTel）是一个开源项目，旨在为分布式系统（如微服务架构）提供一个厂商中立的标准，用于收集、处理和导出遥测数据。它支持开发人员更轻松地分析软件的性能和行为，从而更容易地诊断和排除应用问题。

alb支持上报otel trace 到指定collector上，支持不同的采样策略，支持在ingress级别单独配置是否上报

## 名词解释
| 名词                                                                        | 解释                                                                                                                 |
|-----------------------------------------------------------------------------|--------------------------------------------------------------------------------------------------------------------|
| otel-server                                                                 | 能够收集上报的otel trace的server，比如jaeger就是这样的一个server                                                      |
| trace                                                                       | 上报到otel server的数据                                                                                              |
| attributes                                                                  | 每个trace中，有很多属性，其中有tracer自身的属性 resource attributes. 也有这次trace的请求相关的一些属性 span attributes |
| sampler                                                                     | 每个采样器中可以配置自己的采样策略，来决定是否要上报请求的trace                                                       |
| alb/ft/rule                                                                 | 对应创建的alb资源，frontend 资源，rule资源，可以在这些资源上配置otel，配置会自动继承                                     |
| [hotrod](https://github.com/jaegertracing/jaeger/tree/main/examples/hotrod) | jaeger提供的用来演示如何是用otel的demo,由多个微服务组成.                                                             |
| [hotrod-with-proxy](https://github.com/woodgear/hotrod-with-proxy/blob/master/services/frontend/best_eta.go#L53)                                                           | 通过环境变量指定hotrod内部各微服务地址                                                                               |
## quick demo
![otel-quick-demo](./res/alb-otel.drawio.svg)

如下yaml中部署了一个alb，使用jaeger作为otel-server，hotrod-proxy作为demo backend，通过配置ingress规则，client在请求alb时，流量会转发至hotrod，hotrod自己内部的微服务也通过alb做转发。

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: hotrod
spec:
  replicas: 1
  selector:
    matchLabels:
      service.cpaas.io/name: hotrod
      service_name: hotrod
  template:
    metadata:
      labels:
        service.cpaas.io/name: hotrod
        service_name: hotrod
    spec:
      containers:
        - name: hotrod
          env:
            - name: PROXY_PORT
              value: "80"
            - name: PROXY_ADDR
              value: "otel-alb.default.svc.cluster.local:"
            - name: OTEL_EXPORTER_OTLP_ENDPOINT
              value: "http://jaeger.default.svc.cluster.local:4318"
          image: theseedoaa/hotrod-with-proxy:latest
          imagePullPolicy: IfNotPresent
          command: ["/bin/hotrod","all","-v"]
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: hotrod-frontend
spec:
  ingressClassName: otel-alb
  rules:
  - http:
      paths:
      - backend:
          service:
            name: hotrod
            port:
              number: 8080
        path: /dispatch
        pathType: ImplementationSpecific
      - backend:
          service:
            name: hotrod
            port:
              number: 8080
        path: /frontend
        pathType: ImplementationSpecific
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: hotrod-customer
spec:
  ingressClassName: otel-alb
  rules:
  - http:
      paths:
      - backend:
          service:
            name: hotrod
            port:
              number: 8081
        path: /customer
        pathType: ImplementationSpecific
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: hotrod-route
spec:
  ingressClassName: otel-alb
  rules:
  - http:
      paths:
      - backend:
          service:
            name: hotrod
            port:
              number: 8083
        path: /route
        pathType: ImplementationSpecific
---
apiVersion: v1
kind: Service
metadata:
  name: hotrod
spec:
  internalTrafficPolicy: Cluster
  ipFamilies:
    - IPv4
  ipFamilyPolicy: SingleStack
  ports:
    - name: frontend
      port: 8080
      protocol: TCP
      targetPort: 8080
    - name: customer
      port: 8081
      protocol: TCP
      targetPort: 8081
    - name: router
      port: 8083
      protocol: TCP
      targetPort: 8083
  selector:
    service_name: hotrod
  sessionAffinity: None
  type: ClusterIP
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: jaeger
spec:
  replicas: 1
  selector:
    matchLabels:
      service.cpaas.io/name: jaeger
      service_name: jaeger
  template:
    metadata:
      labels:
        service.cpaas.io/name: jaeger
        service_name: jaeger
    spec:
      containers:
        - name: jaeger
          env:
           - name: LOG_LEVEL
             value: debug
          image: jaegertracing/all-in-one:1.58.1
          imagePullPolicy: IfNotPresent
      hostNetwork: true
      tolerations:
        - operator: Exists
---
apiVersion: v1
kind: Service
metadata:
  name: jaeger
spec:
  internalTrafficPolicy: Cluster
  ipFamilies:
    - IPv4
  ipFamilyPolicy: SingleStack
  ports:
    - name: http
      port: 4318
      protocol: TCP
      targetPort: 4318
  selector:
    service_name: jaeger
  sessionAffinity: None
  type: ClusterIP
---
apiVersion: crd.alauda.io/v2
kind: ALB2
metadata:
  name: otel-alb
spec:
  config:
    loadbalancerName: otel-alb
    otel:
      enable: true
      exporter:
        collector:
          address: "http://jaeger.default.svc.cluster.local:4318"
          request_timeout: 1000
    projects:
    - ALL_ALL
    replicas: 1
    resources:
      alb:
        limits:
          cpu: 200m
          memory: 2Gi
        requests:
          cpu: 50m
          memory: 128Mi
      limits:
        cpu: "1"
        memory: 1Gi
      requests:
        cpu: 50m
        memory: 128Mi
  type: nginx
```
将上述yaml保存为all.yaml
1. 执行 `kubectl apply ./all.yaml`,这一步将部署jaeger，alb，hotrod和测试需要的所有cr

2. 执行`export JAEGER_IP=$(kubectl get po -A -o wide |grep jaeger | awk '{print $7}');echo "http://$JAEGER_IP:16686"` 这一步获得结果为jaeger的访问地址

3. 执行`export ALB_IP=$(kubectl get po -A -o wide|grep otel-alb | awk '{print $7}');echo $ALB_IP` 这一步获取结果为otel-alb的访问地址

4. 执行`curl -v "http://$ALB_IP:80/dispatch?customer=567&nonse="`通过alb来向hotrod发请求,alb会将trace上报到jaeger中，可以打开`"http://$JAEGER_IP:16686"`来查看。
结果应类似于
![jaeger](./res/jaeger.png)
![trace](./res/trace.png)


## 详细配置
## 前提条件
1. 创建/找到要操作的alb 以下称为`<otel-alb>`
2. 确认otel上报数据的server的地址 以下称为`<jaeger-server>`

## 操作步骤
1. `kubectl edit alb2 -n cpaas-system <otel-alb>`
更新alb `<otel-alb>`,在spec.config 上加上如下配置

```yaml
    otel:
      enable: true
      exporter:
        collector:
          address: "<jaeger-server>"
          request_timeout: 1000
```
更新完成之后，这个alb就会默认开启otel，将所有的请求的trace信息都上报到jaeger-server上去。

## 不同的配置
### 在ingress上开启或关闭otel

```yaml
kind: Ingress
metadata:
  annotations:
    nginx.ingress.kubernetes.io/enable-opentelemetry: "true"
```
### 在ingress上开启或关闭otel trust
```yaml
kind: Ingress
metadata:
  annotations:
    nginx.ingress.kubernetes.io/opentelemetry-trust-incoming-span: "true"
```
关闭之后，alb就不会使用请求中的traceid等信息，而是会新创建一个traceid
### 在ingress上设置不同的otel配置
```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  annotations:
    alb.ingress.cpaas.io/otel: >
     {
        "enable": true,
        "exporter": {
            "collector": {
                "address": "http://128.0.0.1:4318",
                "request_timeout": 1000
            }
        }
     }
```

### 完整配置
otel配置的完整shema可以通过crd查看  `kubectl get crd alaudaloadbalancer2.crd.alauda.io -o json|jq ".spec.versions[2].schema.openAPIV3Schema.properties.spec.properties.config.properties.otel"`.
```json
{
    "otel": {
        "enable": true # 开启还是关闭上报
    }
    "exporter": {
        "collector": { # 上报地址
            "address": ""# 支持http/https 支持域名
          },
    },
    "flags": { 
        "hide_upstream_attrs": false # 是否上报alb关于upstream的规则的信息
        "notrust_incoming_span": false # # 是否使用请求中的otel信息
        "report_http_reqeust_header": false # 是否上报请求header 
        "report_http_response_header": false  # 是否上报响应header
    },
    "sampler": {
        "name": "", # 采样策略名，具体见#采样策略
        "options": {
            "fraction": "" # 采样率
            "parent_name": "" # parent_base 采样的parent策略
          },
      },
 }
```
## 配置继承
在alb，ft，rule上都可以通过spec.config.otel设置otel配置。  
配置项的继承顺序为 alb > ft > rule,可以在alb上设置collector地址，在具体的某个ft，或者rule上开启或关闭配置
## 采样策略
### always on
总是上报
### always off
总是不上报
### traceid-ratio
traceid-ratio指的是使用traceid作为是否要trace的依据  
traceparent的格式为 `xx-traceid-xx-flag` 
其中traceid的前16个字符可以看到16进制的32位整数，如果这个整数小于fraction乘以4294967295(2**32-1),则上报。

### parent-base
根据请求的traceparent的flag部分决定是上报，还是用其他sampler策略  
curl -v "http://$ALB_IP/" -H 'traceparent: 00-xx-xx-01' 上报  
curl -v "http://$ALB_IP/" -H 'traceparent:00-xx-xx-02' 不上报  

## attributes
###  resource attributes

| 默认上报            | 解释             |
|---------------------|------------------|
| hostname            | alb pod hostname |
| service.name        | alb 的名字       |
| service.namespace   | alb ns           |
| service.type        | 默认为alb        |
| service.instance.id | alb pod name     |
### span attributes

#### 默认上报
| name                      | 解释                                     |
|---------------------------|------------------------------------------|
| http.status_code          | status code                              |
| http.request.resend_count | 重试次数                                 |
| alb.rule.rule_name        | 这个请求匹配到的规则名                   |
| alb.rule.source_type      | 这个请求匹配到的规则类型，目前只有ingress |
| alb.rule.source_name      | ingress的name                            |
| alb.rule.source_ns        | ingress的ns                              |

#### 默认上报，可以通过flag.hide_upstream_attrs 关掉 
| name                  | 解释               |
|-----------------------|------------------|
| alb.upstream.svc_name | 转发到的svc的name  |
| alb.upstream.svc_ns   | 转发到的svc的ns    |
| alb.upstream.peer     | 转发到pod ip +端口 |

#### 默认不上报，可以通过flag.report_http_reqeust_header 打开
| name                         | 解释       |
|------------------------------|----------|
| http.request.header.<header> | 请求header |
#### 默认不上报，可以通过flag.report_http_response_header 打开

| name                          | 解释       |
|-------------------------------|----------|
| http.response.header.<header> | 响应header |