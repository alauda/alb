# ACP v3.18
## ACP v3.18.1
state: RELEASE
chart: v3.18.2
image: v3.18.2
### change
#### feature
* add support of [[otel]] [otel](https://github.com/alauda/alb/commit/16ee00dd009cda1bd5fb48ad803b48fe5427d2b6)
* add cpaas.io/project label in ingress synced rule.
* add support of [[modsecurity]]
#### other
* tweak github ci. we could build and test in github now
* deploy alb-operator via deployment. do not use csv anymore
* add MonitorDashboard in chart 
* https and authed metrics
* add source name/ns label in rule. (first 63 chars if name/ns is longer than 63)
* api: filter project when list rules
* unquote cookie value when rewrite request header via var
* swap policy
* fix CORS when has multi-domain
* donot use placeholder crt/key