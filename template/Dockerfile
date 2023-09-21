# build nignx/openresty base image
FROM build-harbor.alauda.cn/ops/golang:1.18-alpine3.15 AS tweak_gen
COPY ./ /alb/
ENV GO111MODULE=on
ENV GOPROXY=https://goproxy.cn,direct
ENV CGO_ENABLED=0
ENV GOFLAGS=-buildvcs=false
WORKDIR /alb
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.ustc.edu.cn/g' /etc/apk/repositories && apk update && apk add git gcc musl-dev
RUN cd cmd/utils/tweak_gen && go build -buildmode=pie -ldflags '-w -s -linkmode=external -extldflags=-Wl,-z,relro,-z,now' -v -o /alb/out/tweak_gen alauda.io/alb2/cmd/utils/tweak_gen
RUN ls -alh /alb/out/

FROM build-harbor.alauda.cn/3rdparty/alb-nginx:v1.22.0

# keep it as same as pkg/config/albrun.go
ENV OLD_CONFIG_PATH /etc/alb2/nginx/nginx.conf
ENV NEW_POLICY_PATH /etc/alb2/nginx/policy.new

RUN mkdir -p /alb/nginx
COPY ./template/nginx /alb/nginx
COPY --from=tweak_gen /alb/out/tweak_gen /alb/tools/tweak_gen
RUN chown -R nonroot:nonroot /alb && \ 
chown -R nonroot:nonroot /usr/local/lib/lua && \ 
chmod -R o-rwx /usr/local/lib/lua && \ 
chmod -R g-w /usr/local/lib/lua && \ 
chown -R nonroot:nonroot /usr/local/openresty && \ 
chmod -R o-rwx /usr/local/openresty && \ 
chmod -R g-w /usr/local/openresty && \ 
chmod -R o-rwx /alb && chmod -R g-w /alb && \ 
apk add --no-cache zlib-dev libcap && rm -rf /usr/bin/nc && \ 
chmod 550 /alb/nginx/run-nginx.sh && \ 
setcap CAP_NET_BIND_SERVICE=+eip /usr/local/openresty/nginx/sbin/nginx && \ 
ls -alh /usr/local/openresty/nginx/conf

USER nonroot
WORKDIR /alb
