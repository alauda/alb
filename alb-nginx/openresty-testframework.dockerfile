FROM build-harbor.alauda.cn/3rdparty/alb-nginx:v3.6.0
RUN apk add perl-app-cpanminus wget make build-base perl-dev git neovim bash yq tree
# install yq 4
RUN wget https://github.com/mikefarah/yq/releases/download/v4.12.1/yq_linux_amd64.tar.gz -O - |tar xz && mv ./yq_linux_amd64 /usr/bin/yq
RUN cpanm -v --notest Test::Nginx IPC::Run
RUN git clone https://github.com/openresty/test-nginx.git
RUN git clone https://github.com/ledgetech/lua-resty-http.git && cp lua-resty-http/lib/resty/* /usr/local/openresty/site/lualib/resty/

