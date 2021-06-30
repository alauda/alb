#!/bin/bash

. ./utils.sh


main() {
	dotest "build-harbor.alauda.cn/acp/alb2:fix-acp-6315.2106241007" "k-alb-test"
}

main