UNAME:=$(shell uname)

ifeq ($(UNAME),Linux)
	SED = sed
endif
ifeq ($(UNAME),Darwin)
	SED = gsed
endif

build:
	CGO_ENABLED=0 go build -buildmode=pie -ldflags '-w -s -linkmode=external -extldflags=-Wl,-z,relro,-z,now' -v -o ./bin/alb alauda.io/alb2
	CGO_ENABLED=0 go build -buildmode=pie -ldflags '-w -s -linkmode=external -extldflags=-Wl,-z,relro,-z,now' -v -o ./bin/migrate/init-port-info alauda.io/alb2/migrate/init-port-info

test:
	go test -v -coverprofile=coverage-all.out ./...

get-all-test-coverage: test
	go tool cover -func=coverage-all.out

gen-code:
	rm -rf pkg/client
	GOOS=linux ./hack/update-codegen.sh
	# fix code-generator wrong pluralize, skip fake_alb2 for test
	# for osx, brew install gnu-sed
	find ./pkg/client -name '*.go' -not -name 'fake_alb2.go' -exec grep -l "alb2s" {} \; | xargs ${SED} 's/"alb2s"/"alaudaloadbalancer2"/g' -i

install-controller-gen:
	go get sigs.k8s.io/controller-tools/cmd/controller-gen
	# verified version is 0.7.0
gen-crd:
	rm -rf ./chart/crds &&  controller-gen  crd paths=./pkg/apis/alauda/v1  output:crd:dir=./chart/crds

test-nginx-ci:
	cd alb-nginx && ./actions/test-nginx-in-ci.sh

test-nginx:
	cd alb-nginx && ./actions/test-nginx.sh
fmt:
	go fmt ./...

lint:
	@GOOS=linux gofmt -d ${GOFILES_NOVENDOR} 
	@GOOS=linux gofmt -l ${GOFILES_NOVENDOR} | read && echo "Code differs from gofmt's style" 1>&2 && exit 1 || true
	@GOOS=linux go vet .../..
