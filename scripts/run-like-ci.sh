# run like ci.
docker run --network=host -v $PWD:/acp-alb-test -it build-harbor.alauda.cn/3rdparty/alb-nginx-test:20220407172357 bash -c 'cd /acp-alb-test ;/acp-alb-test/scripts/ci.sh'
