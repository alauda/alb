ARG RESTY_IMAGE_BASE="alpine"
ARG RESTY_IMAGE_TAG="3.13.5"
FROM ${RESTY_IMAGE_BASE}:${RESTY_IMAGE_TAG} AS builder


RUN sed -i 's|dl-cdn.alpinelinux.org|mirrors.cloud.tencent.com|g' /etc/apk/repositories

ARG LUA_RESTY_BALANCER_VERSION="0.03"
ARG LUA_VAR_NGINX_MODULE_VERSION="0.6"
ARG RESTY_IMAGE_BASE="alpine"
ARG RESTY_VERSION="1.19.3.2"
ARG RESTY_OPENSSL_VERSION="1.1.1k"
ARG RESTY_OPENSSL_PATCH_VERSION="1.1.1f"
ARG RESTY_OPENSSL_URL_BASE="https://www.openssl.org/source"
ARG RESTY_PCRE_VERSION="8.44"
ARG RESTY_J="4"
ARG RESTY_CONFIG_OPTIONS="\
    --with-compat \
    --with-file-aio \
    --with-http_addition_module \
    --with-http_auth_request_module \
    --with-http_dav_module \
    --with-http_flv_module \
    --with-http_geoip_module=dynamic \
    --with-http_gunzip_module \
    --with-http_gzip_static_module \
    --with-http_random_index_module \
    --with-http_realip_module \
    --with-http_secure_link_module \
    --with-http_slice_module \
    --with-http_ssl_module \
    --with-http_stub_status_module \
    --with-http_sub_module \
    --with-http_v2_module \
    --with-http_xslt_module=dynamic \
    --with-ipv6 \
    --with-mail \
    --with-mail_ssl_module \
    --with-md5-asm \
    --with-pcre-jit \
    --with-sha1-asm \
    --with-stream \
    --with-stream_ssl_module \
    --with-threads \
    --with-debug \
    --build=ALB \
    "

ARG RESTY_CONFIG_OPTIONS_MORE="--add-module=/tmp/lua-var-nginx-module-${LUA_VAR_NGINX_MODULE_VERSION}"
ARG RESTY_LUAJIT_OPTIONS="--with-luajit-xcflags='-DLUAJIT_NUMMODE=2 -DLUAJIT_ENABLE_LUA52COMPAT'"
# User openresty/luajit2 latest source to fix http://jira.alauda.cn/browse/ACP-5137. see more detail in README.md
ARG RESTR_LUAJIT2_VERSION="886d5f895b8ae19def724677376322b1f7ae2d4a"

ARG RESTY_ADD_PACKAGE_BUILDDEPS=""
ARG RESTY_ADD_PACKAGE_RUNDEPS=""
ARG RESTY_EVAL_PRE_CONFIGURE="curl -fSL https://github.com/api7/lua-var-nginx-module/archive/v${LUA_VAR_NGINX_MODULE_VERSION}.tar.gz -o lua-var-nginx-module-v${LUA_VAR_NGINX_MODULE_VERSION}.tar.gz \
    && tar xzf lua-var-nginx-module-v${LUA_VAR_NGINX_MODULE_VERSION}.tar.gz && rm -rf lua-var-nginx-module-v${LUA_VAR_NGINX_MODULE_VERSION}.tar.gz"
ARG RESTY_EVAL_POST_MAKE="ln -s /usr/local/openresty/bin/opm /usr/local/bin/opm \
    && ln -s /usr/local/openresty/bin/resty /usr/local/bin/resty \
    && opm install thibaultcha/lua-resty-mlcache \
    && opm install xiangnanscu/lua-resty-cookie \
    && curl -fSL https://github.com/openresty/lua-resty-balancer/archive/v${LUA_RESTY_BALANCER_VERSION}.tar.gz -o lua-resty-balancer-v${LUA_RESTY_BALANCER_VERSION}.tar.gz \
    && tar xzf lua-resty-balancer-v${LUA_RESTY_BALANCER_VERSION}.tar.gz && rm -rf lua-resty-balancer-v${LUA_RESTY_BALANCER_VERSION}.tar.gz \
    && cd lua-resty-balancer-${LUA_RESTY_BALANCER_VERSION} \
    && make && make install && cd - \
    && cd lua-var-nginx-module-${LUA_VAR_NGINX_MODULE_VERSION} \
    && cp -r lib/resty/* /usr/local/openresty/lualib/resty && cd -"

# These are not intended to be user-specified
ARG _RESTY_CONFIG_DEPS="--with-pcre \
    --with-cc-opt='-DNGX_LUA_ABORT_AT_PANIC -I/usr/local/openresty/pcre/include -I/usr/local/openresty/openssl/include' \
    --with-ld-opt='-L/usr/local/openresty/pcre/lib -L/usr/local/openresty/openssl/lib -Wl,-rpath,/usr/local/openresty/pcre/lib:/usr/local/openresty/openssl/lib' \
    "

LABEL resty_image_base="${RESTY_IMAGE_BASE}"
LABEL resty_image_tag="${RESTY_IMAGE_TAG}"
LABEL resty_version="${RESTY_VERSION}"
LABEL resty_openssl_version="${RESTY_OPENSSL_VERSION}"
LABEL resty_openssl_patch_version="${RESTY_OPENSSL_PATCH_VERSION}"
LABEL resty_openssl_url_base="${RESTY_OPENSSL_URL_BASE}"
LABEL resty_pcre_version="${RESTY_PCRE_VERSION}"
LABEL resty_config_options="${RESTY_CONFIG_OPTIONS}"
LABEL resty_config_options_more="${RESTY_CONFIG_OPTIONS_MORE}"
LABEL resty_config_deps="${_RESTY_CONFIG_DEPS}"
LABEL resty_add_package_builddeps="${RESTY_ADD_PACKAGE_BUILDDEPS}"
LABEL resty_add_package_rundeps="${RESTY_ADD_PACKAGE_RUNDEPS}"
LABEL resty_eval_pre_configure="${RESTY_EVAL_PRE_CONFIGURE}"
LABEL resty_eval_post_make="${RESTY_EVAL_POST_MAKE}"


# 1) Install apk dependencies
# 2) Download and untar OpenSSL, PCRE, and OpenResty
# 3) Build OpenResty
# 4) Cleanup

RUN apk add --no-cache --virtual .build-deps \
        build-base \
        coreutils \
        gd-dev \
        geoip-dev \
        libxslt-dev \
        linux-headers \
        make \
        perl-dev \
        readline-dev \
        zlib-dev \
        git \
        ${RESTY_ADD_PACKAGE_BUILDDEPS}

RUN apk add --no-cache \
        curl \
        geoip \
        libgcc \
        libxslt \
        zlib \
        ca-certificates \
	    jq iproute2 logrotate openssl tcpdump tshark tini \
        ${RESTY_ADD_PACKAGE_RUNDEPS}
RUN update-ca-certificates \
    && cd /tmp \
    && if [ -n "${RESTY_EVAL_PRE_CONFIGURE}" ]; then eval $(echo ${RESTY_EVAL_PRE_CONFIGURE}); fi \
    && cd /tmp \
    && curl -fSL "${RESTY_OPENSSL_URL_BASE}/openssl-${RESTY_OPENSSL_VERSION}.tar.gz" -o openssl-${RESTY_OPENSSL_VERSION}.tar.gz \
    && tar xzf openssl-${RESTY_OPENSSL_VERSION}.tar.gz \
    && cd openssl-${RESTY_OPENSSL_VERSION} \
    && if [ $(echo ${RESTY_OPENSSL_VERSION} | cut -c 1-5) = "1.1.1" ] ; then \
        echo 'patching OpenSSL 1.1.1 for OpenResty' \
        && curl -s https://raw.githubusercontent.com/openresty/openresty/master/patches/openssl-${RESTY_OPENSSL_PATCH_VERSION}-sess_set_get_cb_yield.patch | patch -p1 ; \
    fi \
    && ./config \
      no-threads shared zlib -g \
      enable-ssl3 enable-ssl3-method \
      --prefix=/usr/local/openresty/openssl \
      --libdir=lib \
      -Wl,-rpath,/usr/local/openresty/openssl/lib \
    && make -j${RESTY_J} \
    && make -j${RESTY_J} install_sw \
    && cd /tmp \
    && curl -fSL https://ftp.pcre.org/pub/pcre/pcre-${RESTY_PCRE_VERSION}.tar.gz -o pcre-${RESTY_PCRE_VERSION}.tar.gz \
    && tar xzf pcre-${RESTY_PCRE_VERSION}.tar.gz \
    && cd /tmp/pcre-${RESTY_PCRE_VERSION} \
    && ./configure \
        --prefix=/usr/local/openresty/pcre \
        --disable-cpp \
        --enable-jit \
        --enable-utf \
        --enable-unicode-properties \
    && make -j${RESTY_J} \
    && make -j${RESTY_J} install \
    && cd /tmp \
    && echo "start build openresty" \
    && curl -fSL https://openresty.org/download/openresty-${RESTY_VERSION}.tar.gz -o openresty-${RESTY_VERSION}.tar.gz \
    && tar xzf openresty-${RESTY_VERSION}.tar.gz \
    ## use custom luajit version,see more details in README.md
    && rm -fr /tmp/openresty-${RESTY_VERSION}/bundle/LuaJIT-2.1-20201027 \
    && git clone https://github.com/openresty/luajit2 \
    && cd ./luajit2 && git checkout ${RESTR_LUAJIT2_VERSION} && cd ../ && mv ./luajit2 /tmp/openresty-${RESTY_VERSION}/bundle/LuaJIT-2.1-20210623 \
    && cd /tmp/openresty-${RESTY_VERSION} \
    && eval ./configure -j${RESTY_J} ${_RESTY_CONFIG_DEPS} ${RESTY_CONFIG_OPTIONS} ${RESTY_CONFIG_OPTIONS_MORE} ${RESTY_LUAJIT_OPTIONS} \
    && make -j${RESTY_J} \
    && make -j${RESTY_J} install \
    && cd /tmp \
    && if [ -n "${RESTY_EVAL_POST_MAKE}" ]; then eval $(echo ${RESTY_EVAL_POST_MAKE}); fi \
    && echo "build openresty over" \
    && rm -rf RESTY_OPENSSL_PATCH_VERSION\
        openssl-${RESTY_OPENSSL_VERSION}.tar.gz openssl-${RESTY_OPENSSL_VERSION} \
        pcre-${RESTY_PCRE_VERSION}.tar.gz pcre-${RESTY_PCRE_VERSION} \
        openresty-${RESTY_VERSION}.tar.gz openresty-${RESTY_VERSION} \
    && apk del .build-deps \
    && addgroup -S nginx \
    && adduser -D -S -h /var/cache/nginx -s /sbin/nologin -G nginx nginx

RUN chmod 755 -R /usr/local/openresty

# Copy everything into a fresh scratch image so that naive CVE scanners don't pick up binaries and libraries
# that have been removed in our later layers.
FROM scratch
COPY --from=builder / /

# Add additional binaries into PATH for convenience
ENV PATH=$PATH:/usr/local/openresty/luajit/bin/:/usr/local/openresty/nginx/sbin/:/usr/local/openresty/bin/
