#!/bin/bash

make static-build
md5sum ./bin/alb
ls -alh ./bin/alb
# run alb pop: while true;do ls -alh ./alb/alb ;md5sum ./alb/alb ;sleep 1s;done
kubectl cp ./bin/alb cpaas-system/$1:/alb/alb -c alb2
