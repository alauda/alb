FROM build-harbor.alauda.cn/ops/golang:1.18-alpine3.15  AS builder

ENV GO111MODULE=on
ENV GOPROXY=https://goproxy.cn,direct
ENV CGO_ENABLED=0
ENV GOFLAGS=-buildvcs=false
COPY . $GOPATH/src/alauda.io/alb2
WORKDIR $GOPATH/src/alauda.io/alb2
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.ustc.edu.cn/g' /etc/apk/repositories && apk update && apk add git gcc musl-dev
RUN go build -buildmode=pie -ldflags '-w -s -linkmode=external -extldflags=-Wl,-z,relro,-z,now' -v -o /out/alb alauda.io/alb2/cmd/alb
RUN go build -buildmode=pie -ldflags '-w -s -linkmode=external -extldflags=-Wl,-z,relro,-z,now' -v -o /out/migrate/init-port-info alauda.io/alb2/migrate/init-port-info
RUN go build -buildmode=pie -ldflags '-w -s -linkmode=external -extldflags=-Wl,-z,relro,-z,now' -v -o /out/operator alauda.io/alb2/cmd/operator

FROM build-harbor.alauda.cn/ops/alpine:3.17.1 AS base
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.ustc.edu.cn/g' /etc/apk/repositories
RUN apk update && apk add --no-cache iproute2 jq libcap && rm -rf /usr/bin/nc

ENV NGINX_TEMPLATE_PATH /alb/ctl/template/nginx/nginx.tmpl
ENV NEW_CONFIG_PATH /etc/alb2/nginx/nginx.conf.new
ENV OLD_CONFIG_PATH /etc/alb2/nginx/nginx.conf
ENV NEW_POLICY_PATH /etc/alb2/nginx/policy.new
ENV INTERVAL 5
ENV BIND_ADDRESS *
ENV GODEBUG=cgo


RUN umask 027 && \
    mkdir -p /alb && \
    mkdir -p /alb/ctl/migrate

COPY run-alb.sh /alb/ctl/run-alb.sh
RUN chmod +x /alb/ctl/run-alb.sh
COPY viper-config.toml /alb/ctl/viper-config.toml
COPY ./template/nginx/nginx.tmpl /alb/ctl/template/nginx/nginx.tmpl
COPY --from=builder /out/alb /alb/ctl/alb
COPY --from=builder /out/operator /alb/ctl/operator
COPY --from=builder /out/migrate/init-port-info /alb/ctl/migrate/init-port-info

RUN chown -R nonroot:nonroot /alb && \
    setcap CAP_SYS_PTRACE=+eip /sbin/ss && \
    chmod -R o-rwx /alb; chmod -R g-w /alb  && \
    chmod 550 /alb/ctl/run-alb.sh && \
    chmod 550 /alb/ctl/migrate/init-port-info && \
    chmod 550 /alb/ctl/alb 
RUN ls /usr/bin |grep nc
USER nonroot
