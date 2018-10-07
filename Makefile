.PHONY: version

version:
	git describe --long > VERSION

images: version test
	docker build -t index.alauda.cn/alaudaorg/alb2:`cat VERSION` -f Dockerfile.nginx.local .

push: images
	docker push index.alauda.cn/alaudaorg/alb2:`cat VERSION`

test:
	go test -cover=true -v ./...