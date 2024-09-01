# ACP v3.18
## ACP v3.18.1
image: NOT_RELEASE  
chart: NOT_RELEASE  
### change
#### feature
* add support of [[otel]] [otel](https://github.com/alauda/alb/commit/16ee00dd009cda1bd5fb48ad803b48fe5427d2b6)
* add cpaas.io/project label in ingress synced rule.
#### other
* tweak github ci. we could build and test in github now.
* deploy alb-operator via deployment. do not use csv anymore.
* add MonitorDashboard in chart .
* add source name/ns label in rule. (first 63 chars if name/ns is longer than 63)
* api: filter project when list rules.
