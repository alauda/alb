FROM build-harbor.alauda.cn/3rdparty/golang:v1.16.3 AS builder

ENV GO111MODULE=on
ENV GOPROXY=https://goproxy.cn,direct
ENV CGO_ENABLED=0
COPY . $GOPATH/src/alauda.io/alb2
WORKDIR $GOPATH/src/alauda.io/alb2
RUN go build -buildmode=pie --ldflags '-w -s -extldflags "-z now"' -v -o /alb alauda.io/alb2

FROM build-harbor.alauda.cn/3rdparty/alb-nginx:v3.6.0
ENV NGINX_BIN_PATH /usr/local/openresty/nginx/sbin/nginx
ENV NGINX_TEMPLATE_PATH /alb/template/nginx/nginx.tmpl
ENV NEW_CONFIG_PATH /usr/local/openresty/nginx/conf/nginx.conf.new
ENV OLD_CONFIG_PATH /usr/local/openresty/nginx/conf/nginx.conf
ENV NEW_POLICY_PATH /usr/local/openresty/nginx/conf/policy.new
ENV INTERVAL 5
ENV SYNC_POLICY_INTERVAL 1
ENV ROTATE_INTERVAL 20
ENV BIND_ADDRESS *
ENV INGRESS_HTTP_PORT 80
ENV INGRESS_HTTPS_PORT 443
ENV GODEBUG=netdns=cgo
ENV ENABLE_PROFILE false
ENV METRICS_PORT 1936
ENV RESYNC_PERIOD 300
ENV ENABLE_GC false
ENV BACKLOG 2048


CMD ["/sbin/tini", "--", "/run.sh"]

RUN mkdir -p /var/log/mathilde && \
    mkdir -p /alb/certificates && \
    mkdir -p /alb/tweak && \
    mkdir -p /var/log/nginx
COPY alauda /etc/logrotate.d/alauda

COPY run.sh /run.sh
RUN chmod +x /run.sh
COPY alb-config.toml /alb/alb-config.toml
COPY ./template/nginx /alb/template/nginx
COPY --from=builder /alb /alb/alb
COPY alauda /etc/logrotate.d/alauda
# some lua module may not upload to opm or not the latest version
COPY ./3rd-lua-module/lib/resty/ /usr/local/openresty/site/lualib/resty/

RUN chmod 755 -R /alb
