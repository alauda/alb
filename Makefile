GOFILES_NOVENDOR=$(shell find . -type f -name '*.go' -not -path "./vendor/*")

.PHONY: version

version:
	git diff --quiet HEAD --
	git describe --long > VERSION

images: gen-code lint test version
	docker build -t index.alauda.cn/alaudaorg/alb2:`cat VERSION` -f Dockerfile.nginx.local .
	docker image prune -f

push: images
	docker push index.alauda.cn/alaudaorg/alb2:`cat VERSION`

release: push
	docker tag index.alauda.cn/alaudaorg/alb2:`cat VERSION` index.alauda.cn/claas/alb2:`cat VERSION`
	docker push index.alauda.cn/claas/alb2:`cat VERSION`

test:
	go test -cover -v ./... -json > test.json
	go test -v -coverprofile=coverage-all.out ./...

gen-code:
	rm -rf pkg/client
	./hack/update-codegen.sh

lint:
	@gofmt -d ${GOFILES_NOVENDOR} 
	@gofmt -l ${GOFILES_NOVENDOR} | read && echo "Code differs from gofmt's style" 1>&2 && exit 1 || true
	@go tool vet ${GOFILES_NOVENDOR}
