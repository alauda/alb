GOFILES_NOVENDOR=$(shell find . -type f -name '*.go' -not -path "./vendor/*")
GO_IMAGE=index.alauda.cn/alaudaorg/alaudabase-alpine-go:1.12.9-alpine3.9.4

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
	GOOS=linux ./hack/update-codegen.sh
	# fix code-generator wrong pluralize, skip fake_alb2 for test
	# for osx, brew install gnu-sed --with-default-names
	find ./pkg/client -name '*.go' -not -name 'fake_alb2.go' -exec grep -l "alb2s" {} \; | xargs sed 's/"alb2s"/"alaudaloadbalancer2"/g' -i

lint:
	@GOOS=linux gofmt -d ${GOFILES_NOVENDOR} 
	@GOOS=linux gofmt -l ${GOFILES_NOVENDOR} | read && echo "Code differs from gofmt's style" 1>&2 && exit 1 || true
	@GOOS=linux go vet .../..
