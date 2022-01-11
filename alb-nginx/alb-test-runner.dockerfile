FROM build-harbor.alauda.cn/3rdparty/alb-nginx:v3.6.1
RUN uname -m 
RUN chmod || true 
RUN pwd
RUN apk update && apk add  gcompat go python3 py3-pip luacheck lua  perl-app-cpanminus wget make build-base perl-dev git neovim bash yq jq tree fd openssl
RUN cpanm -v --notest Test::Nginx IPC::Run 
# install yq 4
RUN wget https://github.com/mikefarah/yq/releases/download/v4.12.1/yq_linux_amd64.tar.gz -O - |tar xz && mv ./yq_linux_amd64 /usr/bin/yq
RUN git clone https://github.com/openresty/test-nginx.git
RUN git clone https://github.com/ledgetech/lua-resty-http.git && cp lua-resty-http/lib/resty/* /usr/local/openresty/site/lualib/resty/
RUN wget -O lua-format https://github.com/Koihik/vscode-lua-format/raw/master/bin/linux/lua-format && chmod a+x ./lua-format &&  mv ./lua-format /usr/bin/ && lua-format --help
RUN pip install crossplane
RUN curl -sSLo envtest-bins.tar.gz "https://go.kubebuilder.io/test-tools/1.19.2/$(go env GOOS)/$(go env GOARCH)" && \
    mkdir -p /usr/local/kubebuilder && \
    tar -C /usr/local/kubebuilder --strip-components=1 -zvxf envtest-bins.tar.gz && \
    rm envtest-bins.tar.gz && \
	ls /usr/local/kubebuilder 