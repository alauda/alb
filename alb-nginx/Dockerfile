# build nignx/openresty base image
ARG OPS_TOOLSETS_TAG=20221011122042
FROM build-harbor.alauda.cn/ops/toolset:${OPS_TOOLSETS_TAG} AS tools

FROM build-harbor.alauda.cn/3rdparty/alb-nginx:v1.22.0

LABEL OPS_TOOLSETS_TAG="${OPS_TOOLSETS_TAG}"

RUN mkdir -p /alb/nginx
COPY ./template/nginx /alb/nginx
COPY --from=tools /usr/local/bin/gosu /usr/local/bin/
RUN ls /alb/nginx && chown -R nonroot:nonroot /alb && \
    apk add --no-cache sudo zlib-dev libcap && \
    mkdir -p /etc/sudoers.d && \
    echo "nonroot ALL=(ALL) NOPASSWD: ALL" > /etc/sudoers.d/nonroot && \
    setcap CAP_NET_BIND_SERVICE=+eip   /usr/local/openresty/nginx/sbin/nginx && \
    chmod 0440 /etc/sudoers.d/nonroot && \
    chmod 550 /alb/nginx/boot-nginx.sh && \
    chmod 550 /alb/nginx/run-nginx.sh && \
    chmod 750 /alb/nginx/lua && \
    find /alb/nginx/lua -type f | xargs -I {} chmod 640 {} && \
    find /alb/nginx -type d | xargs -I {} chmod 750 {}

USER nonroot
WORKDIR /alb