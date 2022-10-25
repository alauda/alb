FROM build-harbor.alauda.cn/ops/golang:1.18-alpine3.15  AS builder

ENV GO111MODULE=on
ENV GOPROXY=https://goproxy.cn,direct
ENV CGO_ENABLED=0
ENV GOFLAGS=-buildvcs=false
COPY . $GOPATH/src/alauda.io/alb2
WORKDIR $GOPATH/src/alauda.io/alb2
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.ustc.edu.cn/g' /etc/apk/repositories && apk update && apk add git gcc musl-dev
RUN go build -buildmode=pie -ldflags '-w -s -linkmode=external -extldflags=-Wl,-z,relro,-z,now' -v -o /alb alauda.io/alb2
RUN go build -buildmode=pie -ldflags '-w -s -linkmode=external -extldflags=-Wl,-z,relro,-z,now' -v -o /migrate/init-port-info alauda.io/alb2/migrate/init-port-info

FROM build-harbor.alauda.cn/ops/alpine:3.16 AS base
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.ustc.edu.cn/g' /etc/apk/repositories
RUN apk update && apk add --no-cache iproute2 jq sudo

ENV NGINX_BIN_PATH /usr/local/openresty/nginx/sbin/nginx
ENV NGINX_TEMPLATE_PATH /alb/ctl/template/nginx/nginx.tmpl
ENV NEW_CONFIG_PATH /etc/alb2/nginx/nginx.conf.new
ENV OLD_CONFIG_PATH /etc/alb2/nginx/nginx.conf
ENV NEW_POLICY_PATH /etc/alb2/nginx/policy.new
ENV INTERVAL 5
ENV SYNC_POLICY_INTERVAL 1
ENV CLEAN_METRICS_INTERVAL 2592000
ENV BIND_ADDRESS *
ENV INGRESS_HTTP_PORT 80
ENV INGRESS_HTTPS_PORT 443
ENV GODEBUG=cgo
ENV ENABLE_GO_MONITOR false
ENV GO_MONITOR_PORT 1937
ENV METRICS_PORT 1936
ENV RESYNC_PERIOD 300
ENV ENABLE_GC false
ENV BACKLOG 2048


RUN umask 027 && \
    mkdir -p /alb && \
    mkdir -p /alb/ctl/migrate

COPY run-alb.sh /alb/ctl/run-alb.sh
RUN chmod +x /alb/ctl/run-alb.sh
COPY viper-config.toml /alb/ctl/viper-config.toml
COPY ./template/nginx/nginx.tmpl /alb/ctl/template/nginx/nginx.tmpl
COPY --from=builder /alb /alb/ctl/alb
COPY --from=builder /migrate/init-port-info /alb/ctl/migrate/init-port-info

RUN chown -R nonroot:nonroot /alb && \
    setcap CAP_SYS_PTRACE=+eip /sbin/ss && \
    mkdir -p /etc/sudoers.d && \
    echo "nonroot ALL=(ALL) NOPASSWD: ALL" > /etc/sudoers.d/nonroot && \
    chmod 0440 /etc/sudoers.d/nonroot && \
    chmod 550 /alb/ctl/run-alb.sh && \
    chmod 550 /alb/ctl/migrate/init-port-info && \
    chmod 550 /alb/ctl/alb && \
    chmod 750 /alb/ctl/template && chmod 750 /alb/ctl/template/nginx && chmod 640 /alb/ctl/template/nginx/nginx.tmpl
USER nonroot
CMD ["/sbin/tini", "--", "/alb/ctl/run-alb.sh"]
