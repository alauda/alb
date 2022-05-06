FROM build-harbor.alauda.cn/ops/golang:1.18-alpine3.15  AS builder

ENV GO111MODULE=on
ENV GOPROXY=https://goproxy.cn,direct
ENV CGO_ENABLED=0
COPY . $GOPATH/src/alauda.io/alb2
WORKDIR $GOPATH/src/alauda.io/alb2
RUN apk update && apk add git gcc musl-dev
RUN go build -buildmode=pie -ldflags '-w -s -linkmode=external -extldflags=-Wl,-z,relro,-z,now' -v -o /alb alauda.io/alb2
RUN go build -buildmode=pie -ldflags '-w -s -linkmode=external -extldflags=-Wl,-z,relro,-z,now' -v -o /migrate/init-port-info alauda.io/alb2/migrate/init-port-info

FROM build-harbor.alauda.cn/ops/alpine:3.15

RUN apk update && apk add --no-cache curl iproute2 jq logrotate openssl

ENV NGINX_BIN_PATH /usr/local/openresty/nginx/sbin/nginx
ENV NGINX_TEMPLATE_PATH /alb/template/nginx/nginx.tmpl
ENV NEW_CONFIG_PATH /usr/local/openresty/nginx/conf/nginx.conf.new
ENV OLD_CONFIG_PATH /usr/local/openresty/nginx/conf/nginx.conf
ENV NEW_POLICY_PATH /usr/local/openresty/nginx/conf/policy.new
ENV INTERVAL 5
ENV SYNC_POLICY_INTERVAL 1
ENV CLEAN_METRICS_INTERVAL 2592000
ENV ROTATE_INTERVAL 20
ENV BIND_ADDRESS *
ENV INGRESS_HTTP_PORT 80
ENV INGRESS_HTTPS_PORT 443
ENV GODEBUG=cgo
ENV ENABLE_PROFILE false
ENV METRICS_PORT 1936
ENV RESYNC_PERIOD 300
ENV ENABLE_GC false
ENV BACKLOG 2048


CMD ["/sbin/tini", "--", "/run.sh"]

RUN mkdir -p /var/log/mathilde && \
    mkdir -p /alb/certificates && \
    mkdir -p /alb/tweak && \
    mkdir -p /migrate && \
    mkdir -p /var/log/nginx
COPY logrotate.conf /etc/logrotate.d/logrotate.conf

COPY run.sh /run.sh
RUN chmod +x /run.sh
COPY viper-config.toml /alb/viper-config.toml
COPY ./template/nginx /alb/template/nginx
COPY --from=builder /alb /alb/alb
COPY --from=builder /migrate/init-port-info /migrate/init-port-info

RUN chmod 755 -R /alb
