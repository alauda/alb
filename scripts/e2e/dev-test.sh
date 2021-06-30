#!/bin/bash

. ./utils.sh

now() {
	date +'%Y-%m-%d-%H-%M-%S'
}

main() {
	image_name=test-alb:$(now)
	image_name=test-alb:2021-07-01-10-07-43
	alb_root=../../
	docker build -f $alb_root/Dockerfile $alb_root -t $image_name
	
	dotest $image_name "test-alb-dev"
}

main