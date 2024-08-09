# 配置OpenTelemetry
## 概述
> OpenTelemetry（OTel）是一个开源项目，旨在为分布式系统（如微服务架构）提供一个厂商中立的标准，用于收集、处理和导出遥测数据。它支持开发人员更轻松地分析软件的性能和行为，从而更容易地诊断和排除应用问题。

alb支持上报otel trace 到指定collector上，支持不同的采样策略，支持在ingress级别单独配置是否上报

## 名词解释
| 名词        | 解释                                                                                                                    |
|-------------|-----------------------------------------------------------------------------------------------------------------------|
| otel server | 能够收集上报的otel trace的server，比如jaeger就是这样的一个server                                                         |
| trace       | 上报到otel server的的数据                                                                                               |
| attributes  | 每个trace中，有很多属性，其中有tracer自身的一个属性 resource attributes 也有这次trace的请求相关的一些属性 span attributes |
| sampler     | 每个采样器中可以配置自己的采样策略，来决定是否要上报请求的trace                                                          |

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
curl -v "http://$ALB_IP:81/" -H 'traceparent: 00-xx-xx-01' 上报  
curl -v "http://$ALB_IP:81/" -H 'traceparent:00-xx-xx-02' 不上报  

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

| 默认上报                  | 解释                                     |
|---------------------------|------------------------------------------|
| http.status_code          | status code                              |
| http.request.resend_count | 重试测试                                 |
| alb.rule.rule_name        | 这个请求匹配到的规则名                   |
| alb.rule.source_type      | 这个请求匹配到的规则类型，目前只有ingress |
| alb.rule.source_name      | ingress的name                            |
| alb.rule.source_ns        | ingress的ns                              |


| 默认上报，可以通过flag.hide_upstream_attrs 关掉 | 解释               |
|------------------------------------------------|------------------|
| alb.upstream.svc_name                          | 转发到的svc的name  |
| alb.upstream.svc_ns                            | 转发到的svc的ns    |
| alb.upstream.peer                              | 转发到pod ip +端口 |

| 默认不上报，可以通过flag.report_http_reqeust_header 打开 | 解释       |
|---------------------------------------------------------|----------|
| http.request.header.<header>                            | 请求header |

| 默认不上报，可以通过flag.report_http_response_header 打开 | 解释       |
|----------------------------------------------------------|----------|
| http.response.header.<header>                            | 响应header |