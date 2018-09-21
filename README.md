# ALB

ALB fetch lb info from Mirana2 and instances info from marathon then config Haproxy or IAAS LB

## Start up

*For haproxy should import from mirana2 before use*

POST http://api.alauda.cn/v1/load_balancers/

`{
    "region_name": "test-region",
    "create_type": "import",
    "type": "haproxy",
    "name": "192.168.0.1",
    "address": "192.168.0.1",
    "address_type": "internal/external"
}`

Docker need network mode *--net=host* and *--privileged=true*

`docker run -d --name=alb --net=host --privileged=true -e NAMESPACE=claasv1 \
                            -e IAAS_REGION=cn-north-1 \
                            -e SECRET_ACCESS_KEY=wRFZTrXCHziEb9D0gjlaqg5ZorFm+YgRJDmWKgZg \
                            -e REGION_NAME=crawlertest1 \
                            -e JAKIRO_ENDPOINT=http://182.92.204.227:31051 \
                            -e LB_TYPE=haproxy \
                            -e TOKEN=4a2aa78550d6ead018296e23e03cf974f1632db5 \
                            -e MARATHON_SERVER=http://54.223.101.177:8081 \
                            -e ACCESS_KEY=AKIAP44HLE4PAUKHRO4A \
                            -e NAME=mirana2-ha \
                            alb:latest`

## Env var

### All required

1. NAMESPACE, REGION_NAME, JAKIRO_ENDPOINT, TOKEN

### All optional

1. RELOAD_INTERVAL, CERTIFICATE_DIRECTORY, CERTIFICATE_LOAD_INTERVAL, CERTIFICATE_VERIFY

### MARATHON required

1. MARATHON_SERVER

### MARATHON optional

1. 	MARATHON_USERNAME, MARATHON_PASSWORD, MARATHON_TIMEOUT

### KUBERNETES required

1. KUBERNETES_SERVER, KUBERNETES_BEARERTOKEN

### KUBERNETES optional

1. KUBERNETES_TIMEOUT

### For haproxy

1. LB_TYPE: haproxy
2. NAME: related to name in mirana2

### For cloud load balancer

1. IAAS_REGION, SECRET_ACCESS_KEY, ACCESS_KEY: need for config cloud load balancer.
2. LB_TYPE: support elb and slb