FROM index.alauda.cn/alaudaorg/alb-nginx-base:v0.0.6

ENV NGINX_BIN_PATH /usr/local/openresty/nginx/sbin/nginx
ENV NGINX_TEMPLATE_PATH /alb/template/nginx/nginx.tmpl
ENV NEW_CONFIG_PATH /usr/local/openresty/nginx/conf/nginx.conf.new
ENV OLD_CONFIG_PATH /usr/local/openresty/nginx/conf/nginx.conf
ENV NEW_POLICY_PATH /usr/local/openresty/nginx/conf/policy.new
ENV BIND_ADDRESS *

COPY template/nginx /alb/template/nginx
COPY conf/nginx.tmpl /alb/template/nginx/nginx.tmpl
