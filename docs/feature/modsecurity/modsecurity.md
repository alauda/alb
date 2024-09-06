# 配置 ModSecurity

## 概述
> ModSecurity 是一个开源的 Web 应用防火墙 (WAF)，旨在保护 Web 应用免受恶意攻击。它由开源社区维护，支持多种编程语言和 Web 服务器。

ALB 支持配置 ModSecurity，允许在入口级别单独配置以启用或禁用它。

## 术语
| 术语              | 解释                                                                                                               |
|------------------|-------------------------------------------------------------------------------------------------------------------|
| modsecurity      | ModSecurity 是一个开源的 Web 应用防火墙 (WAF)，旨在保护 Web 应用免受恶意攻击。                                      |
| owasp-core-rules | OWASP 核心规则集是一个开源规则集，用于检测和防止常见的 Web 应用攻击。                                               |

## 快速演示

以下是一个 YAML 示例。
1. 部署一个名为 waf-alb 的 ALB。
2. 部署一个演示后端应用：hello。
3. 部署一个定义了 `/waf-enable` 路由的入口 ing-waf-enable，并设置 modsecurity 规则，阻止任何查询参数 test 值包含 test 的请求。

```yaml
apiVersion: crd.alauda.io/v2
kind: ALB2
metadata:
  name: waf-alb
spec:
  config:
    loadbalancerName: waf-alb
    projects:
    - ALL_ALL
    replicas: 1
  type: nginx
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  annotations:
    nginx.ingress.kubernetes.io/enable-modsecurity: "true"
    nginx.ingress.kubernetes.io/modsecurity-transaction-id: "$request_id"
    nginx.ingress.kubernetes.io/modsecurity-snippet: |
        SecRuleEngine On
        SecRule ARGS:test "@contains test" "id:1234,deny,log"
  name: ing-waf-enable
spec:
  ingressClassName: waf-alb
  rules:
  - http:
      paths:
      - backend:
          service:
            name: hello
            port:
              number: 80
        path: /waf-enable
        pathType: ImplementationSpecific
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: ing-waf-normal
spec:
  ingressClassName: waf-alb
  rules:
  - http:
      paths:
      - backend:
          service:
            name: hello
            port:
              number: 80
        path: /waf-not-enable
        pathType: ImplementationSpecific
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: hello
spec:
  replicas: 1
  selector:
    matchLabels:
      service.cpaas.io/name: hello
      service_name: hello
  template:
    metadata:
      labels:
        service.cpaas.io/name: hello
        service_name: hello
    spec:
      containers:
      - name: hello-world
        image: docker.io/hashicorp/http-echo
        imagePullPolicy: IfNotPresent
---
apiVersion: v1
kind: Service
metadata:
  name: hello
spec:
  internalTrafficPolicy: Cluster
  ipFamilies:
    - IPv4
  ipFamilyPolicy: SingleStack
  ports:
    - name: http
      port: 80
      protocol: TCP
      targetPort: 5678
  selector:
    service_name: hello
  sessionAffinity: None
  type: ClusterIP
```

## 详细配置

### ingress-nginx 兼容注解
| 注解                                                   | 类型   | 适用对象          |
|--------------------------------------------------------|--------|-------------------|
| "nginx.ingress.kubernetes.io/enable-modsecurity"       | bool   | ingress           |
| "nginx.ingress.kubernetes.io/enable-owasp-core-rules"  | bool   | ingress           |
| "nginx.ingress.kubernetes.io/modsecurity-transaction-id" | string | ingress           |
| "nginx.ingress.kubernetes.io/modsecurity-snippet"      | string | ingress, alb, ft, rule |

### ALB 特殊注解
| 注解                                     | 类型   | 适用对象 | 描述                              |
|------------------------------------------|--------|----------|----------------------------------|
| "alb.modsecurity.cpaas.io/use-recommand" | bool   | ingress  | 同 modsecurity.useRecommand      |
| "alb.modsecurity.cpaas.io/cmref"         | string | ingress  | 同 modsecurity.cmRef             |

### CR
你可以在 ALB FT RULE 中启用和配置 modsecurity。它们使用相同的结构。

```json5
{ 
 "modsecurity": {
   "enable": true,         // 启用或禁用 modsecurity
   "transactionId": "$xx", // 使用来自 nginx 的 ID
   "useCoreRules": true,   // 添加 `modsecurity_rules_file /etc/nginx/owasp-modsecurity-crs/nginx-modsecurity.conf;`
   "useRecommand": true,   // 添加 `modsecurity_rules_file /etc/nginx/modsecurity/modsecurity.conf;`
   "cmRef": "$ns/$name#$section" // 从 configmap 添加配置
 }
}
```

### 配置覆盖
顺序为 rule, ft, alb。

如果且仅当 rule 中没有 modsecurity 配置时，它将尝试在 ft 中查找配置。如果 ft 中没有配置，它将使用 alb 中的配置。

### ModSecurity 和 CoreRuleSet 版本
ModSecurity: v3.0.13  
coreruleset: v4.4.0