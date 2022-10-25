#!/bin/sh
umask 027
mkdir ./nginx_temp
openssl dhparam -dsaparam -out /etc/alb2/nginx/dhparam.pem 2048
if [ ! -f /etc/alb2/nginx/nginx.conf ]; then
  echo "copy init nginx.conf"
  cp /alb/nginx/nginx.conf /etc/alb2/nginx/nginx.conf
  cat  /etc/alb2/nginx/nginx.conf
fi

nginx -g "daemon off;" -c /etc/alb2/nginx/nginx.conf -e /dev/stderr -p $PWD/nginx_temp
