# ALB

ALB fetch lb info from Mirana2 and instances info from marathon then config nginx or IAAS LB

## Set up
Install alb2 to cluster from jakiro load balancer UI.

## Env var

### All required

1. NAMESPACE, NAME

### All optional

1. RELOAD_INTERVAL, CERTIFICATE_DIRECTORY, CERTIFICATE_LOAD_INTERVAL, CERTIFICATE_VERIFY

### KUBERNETES optional

1. KUBERNETES_TIMEOUT

### For nginx

1. LB_TYPE: nginx
2. NAME: related to name in mirana2

### For cloud load balancer(deprecated)

1. IAAS_REGION, SECRET_ACCESS_KEY, ACCESS_KEY: need for config cloud load balancer.
2. LB_TYPE: support elb and slb
