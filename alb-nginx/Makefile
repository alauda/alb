.PHONY: clean run lint unitest test build-ci build-nginx build bundle version

CI_IMAGE=index.alauda.cn/alaudaorg/alb-nginx-test:t2

version:
	# Exit if there is any uncommit change
	git describe --long > VERSION

clean:
	docker-compose down

int-test: clean
	# FIXME: intergration test
	docker-compose up -d
	docker run --rm --net=host --name=luatest -v $(PWD):/alb-nginx ${CI_IMAGE} busted -v -c test/integration

lint:
	docker run --rm --name=luacheck -v $(PWD):/alb-nginx -v $(PWD)/../template/nginx/lua:/alb-nginx/lua ${CI_IMAGE} luacheck .

unit-test:
	patch -d../template/nginx/lua/ -p4 < test_patch
	docker run --rm --name=luatest -v $(PWD):/alb-nginx -v $(PWD)/../template/nginx/lua:/alb-nginx/lua -v $(PWD)/busted:/usr/local/bin/busted ${CI_IMAGE}  busted -v -c test/unit
	git checkout ../template/nginx/lua/operation.lua

coverage:
	luacov template/nginx/lua/certs.lua
	cat luacov.report.out

test: lint unit-test int-test coverage

build-ci:
	docker build -t ${CI_IMAGE} -f Dockerfile.ci .

build-nginx: version
	docker buildx build --platform linux/amd64,linux/arm64 -t harbor-b.alauda.cn/acp/alb-nginx:`cat VERSION` -f Dockerfile .  --push

build: build-nginx

bundle: test build
