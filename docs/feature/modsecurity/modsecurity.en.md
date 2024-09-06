# Configuring ModSecurity
## Overview
> ModSecurity is an open-source Web Application Firewall (WAF) designed to protect web applications from malicious attacks. It is maintained by an open-source community and supports multiple programming languages and web servers.

ALB supports configuring ModSecurity, allowing individual configuration at the ingress level to enable or disable it.

## Terminology
| Term             | Explanation                                                                                                               |
|------------------|---------------------------------------------------------------------------------------------------------------------------|
| modsecurity      | ModSecurity is an open-source Web Application Firewall (WAF) designed to protect web applications from malicious attacks. |
| owasp-core-rules | The OWASP Core Rule Set is an open-source rule set used to detect and prevent common attacks on web applications.         |

## Quick Demo

The following yaml.
1. deploys an ALB, named as waf-alb.
2. deploy a demo backend app: hello.
4. deploy a ingress ing-waf-enable which define route `/waf-enable`. and set modsecurity rule which will block any req that query parameters test value contains test

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
## detail config
### ingress-nginx compatible annotation
| Annotation                                               | Type   | Applies To          |
|----------------------------------------------------------|--------|---------------------|
| "nginx.ingress.kubernetes.io/enable-modsecurity"         | bool   | ingress             |
| "nginx.ingress.kubernetes.io/enable-owasp-core-rules"    | bool   | ingress             |
| "nginx.ingress.kubernetes.io/modsecurity-transaction-id" | string | ingress             |
| "nginx.ingress.kubernetes.io/modsecurity-snippet"        | string | ingress,alb,ft,rule |

### alb special annotation
| Annotation                               | Type   | Applies To | Description                      |
| ---------------------------------------- | ------ | ---------- | -------------------------------- |
| "alb.modsecurity.cpaas.io/use-recommand" | bool   | ingress    | same as modsecurity.useRecommand |
| "alb.modsecurity.cpaas.io/cmref"         | string | ingress    | same as modsecurity.cmRef        |

### cr
you could enable and config modsecurity in alb ft rule. they are use same struct.

```json5
{ 
 "modsecurity": {
   "enable": true,         // enable or disable modsecurity
   "transactionId": "$xx", // use id frmo nginx 
   "useCoreRules": true,   // add `modsecurity_rules_file /etc/nginx/owasp-modsecurity-crs/nginx-modsecurity.conf;`
   "useRecommand": true,    // add `modsecurity_rules_file /etc/nginx/modsecurity/modsecurity.conf;`
   "cmRef": "$ns/$name#$section"     // add config from configmap
 }
}
```
### config overwrite
the order are rule ft,alb. 

if and only if there no modsecurity config in rule, then it will try to find config in ft. and if no config in ft, it will use config in alb.
### modsecuriry and coreruleset version
modsecuriry: v3.0.13  
coreruleset: v4.4.0