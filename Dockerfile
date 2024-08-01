ARG GO_BUILD_BASE=docker-mirrors.alauda.cn/library/golang:1.22.5-alpine
ARG OPENRESTY_BASE=build-harbor.alauda.cn/3rdparty/alb-nginx:v1.25.3
ARG RUN_BASE=build-harbor.alauda.cn/ops/alpine:3.20

FROM ${GO_BUILD_BASE} AS go_builder

ENV GO111MODULE=on
ENV GOPROXY=https://goproxy.cn,direct
COPY ./ /alb/
WORKDIR /alb
ENV GOFLAGS=-buildvcs=false
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.ustc.edu.cn/g' /etc/apk/repositories && apk update && apk add git gcc musl-dev
RUN go build -buildmode=pie -ldflags '-w -s -linkmode=external -extldflags=-Wl,-z,relro,-z,now' -v -o /out/alb alauda.io/alb2/cmd/alb
RUN go build -buildmode=pie -ldflags '-w -s -linkmode=external -extldflags=-Wl,-z,relro,-z,now' -v -o /out/migrate/init-port-info alauda.io/alb2/migrate/init-port-info
RUN go build -buildmode=pie -ldflags '-w -s -linkmode=external -extldflags=-Wl,-z,relro,-z,now' -v -o /out/operator alauda.io/alb2/cmd/operator
RUN go build -buildmode=pie -ldflags '-w -s -linkmode=external -extldflags=-static' -v -o /out/albctl alauda.io/alb2/cmd/utils/albctl
RUN go build -buildmode=pie -ldflags '-w -s -linkmode=external -extldflags=-Wl,-z,relro,-z,now' -v -o /out/tweak_gen alauda.io/alb2/cmd/utils/tweak_gen
RUN ldd /out/albctl || true

FROM ${OPENRESTY_BASE} AS base
WORKDIR /tmp/
COPY ./template/actions /tmp/
# install our lua dependency
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.ustc.edu.cn/g' /etc/apk/repositories && \ 
apk add --no-cache --virtual .builddeps luarocks5.1 lua5.1 lua5.1-dev bash perl curl build-base make unzip && \ 
cp /usr/bin/luarocks-5.1 /usr/bin/luarocks && \ 
ls /tmp && bash /tmp/alb-nginx-install-deps.sh /usr/local/openresty && \ 
apk del .builddeps build-base make unzip && cd / && rm -rf /tmp && rm /usr/bin/luarocks && rm /usr/bin/nc

# tweak files
FROM scratch
## openresty as base image
COPY --from=base / /
COPY ./template/nginx /alb/nginx
COPY ./template/nginx/nginx.tmpl /alb/ctl/template/nginx/nginx.tmpl
COPY run-alb.sh /alb/ctl/run-alb.sh
COPY --from=go_builder /out/tweak_gen /alb/tools/tweak_gen
COPY --from=go_builder /out/alb /alb/ctl/alb
COPY --from=go_builder /out/migrate /alb/ctl/tools/
COPY --from=go_builder /out/operator /alb/ctl/operator
COPY --from=go_builder /alb/migrate/backup /alb/ctl/tools/backup
COPY --from=go_builder /out/albctl /alb/ctl/tools/albctl

ENV PATH=$PATH:/usr/local/openresty/luajit/bin:/usr/local/openresty/nginx/sbin:/usr/local/openresty/bin:/usr/local/openresty/openssl/bin/
ENV NGINX_TEMPLATE_PATH /alb/ctl/template/nginx/nginx.tmpl
ENV NEW_CONFIG_PATH /etc/alb2/nginx/nginx.conf.new
ENV OLD_CONFIG_PATH /etc/alb2/nginx/nginx.conf
ENV NEW_POLICY_PATH /etc/alb2/nginx/policy.new

# shutdown nginx gracefully
STOPSIGNAL SIGQUIT
# libcap: tweak file capability
# zlib-dev: policy-zip
# iproute2: ss
RUN umask 027 && \ 
sed -i 's/dl-cdn.alpinelinux.org/mirrors.ustc.edu.cn/g' /etc/apk/repositories && \ 
apk add --no-cache zlib-dev libcap iproute2 yq jq curl bash && \ 
mkdir -p /alb/ctl/tools && \ 
mkdir -p /alb/nginx && \ 
echo "build" && chown -R nonroot:nonroot /alb && \ 
chmod +x /alb/ctl/run-alb.sh && \ 
chmod -R o-rwx /alb && \ 
chmod -R g-w /alb && \ 
chmod 550 /alb/ctl/run-alb.sh && \ 
chmod 550 /alb/ctl/tools/init-port-info && \ 
chmod 550 /alb/ctl/tools/albctl && \ 
chmod 550 /alb/ctl/alb && \ 
chown -R nonroot:nonroot /usr/local/openresty && \ 
chmod -R o-rwx /usr/local/openresty && \ 
chmod -R g-w /usr/local/openresty && \ 
chmod -R o-rwx /alb && chmod -R g-w /alb && \ 
chmod 550 /alb/nginx/run-nginx.sh && \ 
ls -alh /usr/local/openresty/nginx/conf && \ 
rm -rf /usr/bin/nc && ls /usr/bin | grep nc && if command -v nc; then exit; fi && \ 
setcap CAP_SYS_PTRACE=+eip /sbin/ss && \ 
setcap CAP_NET_BIND_SERVICE=+eip /usr/local/openresty/nginx/sbin/nginx && \ 
getcap /sbin/ss && \ 
getcap /usr/local/openresty/nginx/sbin/nginx && \ 
true
USER nonroot
