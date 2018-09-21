# CHANGELOG

Given a version number format MAJOR.MINOR.PATCH, increment the:

1. MAJOR version when you make incompatible API changes,
2. MINOR version when you add functionality in a backwards-compatible manner,
3. PATCH version when you make backwards-compatible bug fixes.

## 1.7.1
1. fix: always create NodePort to avoid service lost.
2. fix: order Backend and BackendGroup to avoid unnecessary reload.
3. fix: modify timeout to resolve stale haproxy process.
4. fix: haproxy template bug.
5. fix: haproxy use roundrobin to enable weight.
6. fix: do not delete nodeports created by user.
7. fix: certs in projects can not be downloaded.
4. chore: update haproxy to 1.8.1 and remove iptables parts.
5. chore: use protobuf protocol client-go.

## 1.7.0
1. feat: use new mirana2 api
2. feat: support nginx grey deployment rules
3. feat: support new label style of k8s
4. fix: glog not remove files
5. fix: haproxy tcp backend has no tcp mode and cookie for tcp backend
6. fix: haproxy remove http-server-close option as we already use keepalive mode
7. fix: remove unready nodes from nodeport
8. chore: add more detail log for command error and config change

## 1.6.0
1. feat: Haproxy support session affinity by source ip hash or cookie.
2. feat: tuning sys parameter to improve performance.
3. fix: log related bugs.
4. fix: iptables related bugs.
5. fix: remove down instance logical.
6. fix: glog not rotate.
7. chore: update golang to 1.9.2.

## 1.5.0
1. feat: Haproxy can write post body to access log.
2. feat: Haproxy and nginx can use endpoint as backends.
3. fix: Haproxy can find the right service when Host is not in header.
4. fix: Alb will create a nodeport for every container port and share it between alb-haproxy and alb-xlb.
5. chore: Update Kubernetes SDK to v4.0.0.

## 1.4.0

1. feat: ALB supports Nginx type load balancer.
2. feat: Haproxy supports regex url.
3. feat: ELB supports HTTPS,
4. fix: NodePort not updated if same service changes bind ports to load balancer.
5. fix: SLB ignores UDP and HTTPS listener.
6. fix: Haproxy will return 404 if no server for this request.
7. chore: Update Kubernetes SDK to v2.0.0.
8. chore: Update Haproxy to 1.7.9.

## 1.3.4

1. feat: SLB HTTPS support.
2. fix: Start haproxy even no service bind to it.

## 1.3.3

1. feat: Separate access and error log.
2. fix: Disable rsyslog for alb-xlb.

## 1.3.2

1. feat: Support Haproxy url routing

## 1.3.1

1. fix: Use logrotate to rotate haproxy.log
2. chore: Update haproxy to 1.7.8

## 1.3.0

1. feat: Support SLB url routing
2. fix: Remove $from-host-ip log dir
3. chore: Update haproxy to 1.7.7
4. chore: Remove unused Dockerfile

## 1.2.2(2017/6/23)

1. fix: Add timeout for jakiro request and whole reload process to prevent alb hang and not reload config.
2. refactor: Remove vim, bash and upx to minimize size from 77M to 24M
3. refactor: Use golang 1.8.3

## 1.2.1(2017/6/2)

1. fix: Insert a blank line between public certificate and private key.
2. feature: Verification for certificate can be customized.

## 1.2.0(2017/5/23)

1. feature: Support https listener for haproxy.

## 1.1.2(2017/5/18)

1. fix: Used cgo dns.
2. feat: Rotate log files.
3. feat: Only redirect error log to stdout.

## 1.1.1(2017/4/28)

1. fix: Only manage nodeport service for kubernetes.
2. fix: If no service bind to haproxy avoid report error.
3. fix: Use channel to avoid elb update conflict.
4. fix: Find serverid should use private IP for Aliyun VPC server.
5. fix: Hostname should be translated to IP.

## 1.1.0(2017/4/12)

1. feat: Update Haproxy to 1.7.5.
2. feat: Add slb cache to avoid frequently call ALI api.
3. fix: Reload multi haproxy failed.
4. fix: Minimize log.