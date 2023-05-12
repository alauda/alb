#!/bin/bash

function init() {
  yum update
  yum install docker
  # make sure docker work
  docker pull registry.alauda.cn:60080/acp/alb-nginx:v3.12.2
  # musl
  #   https://rhel.pkgs.org/7/forensics-x86_64/musl-libc-1.2.1-1.el7.x86_64.rpm.html
  # kind
  # helm
  # kubectl
  # yq

}
