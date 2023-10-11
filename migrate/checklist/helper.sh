#!bin/bash
##日志函数
function log::err() {
  printf "[$(date +'%Y-%m-%dT%H:%M:%S')]: \033[31mERROR: \033[0m$@\n"
}

function log::info() {
  printf "[$(date +'%Y-%m-%dT%H:%M:%S')]: \033[32mINFO: \033[0m$@\n"
}
