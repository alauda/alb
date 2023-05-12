FROM build-harbor.alauda.cn/acp/alb-nginx:local
USER root
RUN sh -c "apk update && apk add luacheck lua perl-app-cpanminus wget curl make build-base perl-dev git neovim bash yq jq tree fd openssl &&\
    cpanm --mirror-only --mirror https://mirrors.tuna.tsinghua.edu.cn/CPAN/ -v --notest Test::Nginx IPC::Run"