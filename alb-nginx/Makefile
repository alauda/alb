.PHONY: clean run lint unitest test build-ci build-nginx build push bundle version

version:
	git diff --quiet HEAD --
	git rev-parse HEAD > VERSION

clean:
	docker-compose down

int-test: clean
	docker-compose up -d
	docker run --rm --net=host --name=luatest -v $(PWD):/alb-nginx luaci busted -v -c test/integration

lint:
	docker run --rm --name=luacheck -v $(PWD):/alb-nginx luaci luacheck .

unit-test:
	docker run --rm --name=luatest -v $(PWD):/alb-nginx luaci busted -v -c test/unit

coverage:
	luacov template/nginx/lua/certs.lua
	cat luacov.report.out

test: lint unit-test int-test coverage

build-ci:
	docker build -t luaci -f Dockerfile.ci .

build-nginx: version
	docker build -t index.alauda.cn/alaudaorg/alb-nginx:`cat VERSION` -f Dockerfile.nginx .

build: build-nginx

push:
	docker push index.alauda.cn/alaudaorg/alb-nginx:`cat VERSION`

bundle: test build push
