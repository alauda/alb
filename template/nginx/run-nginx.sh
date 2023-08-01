#!/bin/sh

if [ -n "$TAIL_MODE" ]; then
  echo "tail mode wait forever"
  tail -f /dev/null
fi

umask 027
mkdir -p /alb/nginx/run
openssl dhparam -dsaparam -out /etc/alb2/nginx/dhparam.pem 2048
cfg_path=$OLD_CONFIG_PATH
if [ ! -f $cfg_path ]; then
  echo "copy init nginx.conf"
  cp /alb/nginx/nginx.conf $cfg_path
  cat  $cfg_path
fi

nginx -g "daemon off;" -c /etc/alb2/nginx/nginx.conf -e /dev/stderr -p /alb/nginx/run
