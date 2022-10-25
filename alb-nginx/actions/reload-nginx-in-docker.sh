#!/bin/bash
set -x
ps
ps | grep 'nginx -c' | grep -v 'grep' | awk '{print $1}' | xargs -I{} kill -9 {}
nginx -c /t/servroot/conf/nginx.conf
ps
