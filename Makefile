UNAME:=$(shell uname)

ifeq ($(UNAME),Linux)
	SED = sed
endif
ifeq ($(UNAME),Darwin)
	SED = gsed
endif

.PHONY: test

# TODO this is not a "fully" static build, it still use linux-vdso libc.so ld-xx.so
# for development not for ci.
static-build:
	rm -rf ./bin/ || true
	CGO_ENABLED=0 go build -v -buildmode=pie -ldflags '-w -s -linkmode=external -extldflags=-Wl,-z,relro,-z,now' -v -o ./bin/alb alauda.io/alb2 && ldd ./bin/alb
	CGO_ENABLED=0 go build -v -buildmode=pie -ldflags '-w -s -linkmode=external -extldflags=-Wl,-z,relro,-z,now' -v -o ./bin/migrate/init-port-info alauda.io/alb2/migrate/init-port-info

# used in ci
test: go-unit-test

go-unit-test: go-lint
	go test -v -coverprofile=coverage-all.out `go list ./... |grep -v e2e`

install-envtest:
	bash -c 'source ./scripts/alb-dev-actions.sh && install-envtest'

all-e2e-envtest: install-envtest
	bash -c 'source ./scripts/alb-dev-actions.sh && alb-run-all-e2e-test'

one-e2e-envtest: install-envtest
	cp ./alb-config.toml ./test/e2e
	# due the limit of alb we could only run one test each time
	ENV_TEST_KUBECONFIG=/tmp/env-test.kubecofnig ALB_LOG_LEVEL=9 DEV_MODE=true ginkgo -v ./test/e2e

go-coverage: test
	bash -c 'source ./scripts/alb-dev-actions.sh && alb-go-coverage'

gen-code:
	rm -rf pkg/client
	GOOS=linux ./hack/update-codegen.sh
	# fix code-generator wrong pluralize, skip fake_alb2 for test
	# for osx, brew install gnu-sed
	find ./pkg/client -name '*.go' -not -name 'fake_alb2.go' -exec grep -l "alb2s" {} \; | xargs ${SED} 's/"alb2s"/"alaudaloadbalancer2"/g' -i


# find or download controller-gen
# download controller-gen if necessary
controller-gen:
# verified controller-gen version is 0.7.0
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.7.0 ;\
	}
CONTROLLER_GEN=$(shell which controller-gen)
endif

gen-crd: controller-gen
	rm -rf ./chart/crds &&  controller-gen  crd paths=./pkg/apis/alauda/v1  output:crd:dir=./chart/crds

test-nginx-in-ci:
	cd alb-nginx && ./actions/test-nginx-in-ci.sh

test-nginx:
	cd alb-nginx && ./actions/test-nginx.sh

test-nginx-and-clean:
	cd alb-nginx && ./actions/test-nginx.sh
	sudo rm -rf ./alb-nginx/t/servroot || true

go-fmt:
	go fmt ./...

go-lint:
	gofmt -l .
	[ "`gofmt -l .`" = "" ]
	go vet .../..

install-lua-check:
	# you need install [luaver](https://github.com/DhavalKapil/luaver) first
	yes | luaver install 5.4.3
	luaver current
	lua -v
	luarocks install luacheck
	luacheck -v

lua-check:
	luacheck ./template/nginx/lua

lua-format-check:
	bash -c 'source ./scripts/alb-dev-actions.sh && alb-lua-format-check'

lua-format-format:
	bash -c 'source ./scripts/alb-dev-actions.sh && alb-lua-format-format'

lua-lint: lua-check lua-format-check

lua-test: lua-lint test-nginx-and-clean

init-git-hook:
	bash -c 'source ./scripts/alb-dev-actions.sh && alb-init-git-hook'
test-all: lua-test go-unit-test all-e2e-envtest