UNAME:=$(shell uname)

ifeq ($(UNAME),Linux)
	SED = sed
endif
ifeq ($(UNAME),Darwin)
	SED = gsed
endif

.PHONY: test

build:
	CGO_ENABLED=0 go build -buildmode=pie -ldflags '-w -s -linkmode=external -extldflags=-Wl,-z,relro,-z,now' -v -o ./bin/alb alauda.io/alb2
	CGO_ENABLED=0 go build -buildmode=pie -ldflags '-w -s -linkmode=external -extldflags=-Wl,-z,relro,-z,now' -v -o ./bin/migrate/init-port-info alauda.io/alb2/migrate/init-port-info

test: unit-test

unit-test:
	go test -v -coverprofile=coverage-all.out `go list ./... |grep -v e2e`

envtest:
	bash -c 'source ./scripts/alb-dev-actions.sh && install-envtest'

e2e-envtest: envtest
	cp ./alb-config.toml ./test/e2e
	ENV_TEST_KUBECONFIG=/tmp/env-test.kubecofnig ALB_LOG_LEVEL=9 ginkgo -v ./test/e2e

get-all-test-coverage: test
	go tool cover -func=coverage-all.out

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

test-nginx-ci:
	cd alb-nginx && ./actions/test-nginx-in-ci.sh

test-nginx:
	cd alb-nginx && ./actions/test-nginx.sh

fmt:
	go fmt ./...

lint:
	gofmt -d ./
	test -z $(gofmt -l ./)
	go vet .../..
