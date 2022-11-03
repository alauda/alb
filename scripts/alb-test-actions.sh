#!/bin/bash
# shellcheck disable=SC2120,SC2155,SC2181

function alb-build-e2e-test() {
  local ginkgoCmds=""
  for suite_test in $(find ./test/e2e -name suite_test.go); do
    local suite=$(dirname "$suite_test")
    local suiteName=$(basename "$suite")
    local ginkgoTest="$suite/$suiteName.test"
    ginkgo build $suite
    if [ $? != 0 ]; then
      echo "build $suite failed"
      return 1
    fi
  done
}

function alb-list-e2e-testcase() {
  export ALB_IGNORE_FOCUS="true"
  alb-build-e2e-test >/dev/null
  local ginkgoCmds=""
  # 生成能被直接运行的ginkgo的command.
  # 找到所有的test_suite
  for suite_test in $(find ./test/e2e -name suite_test.go); do
    local suite=$(dirname "$suite_test")
    local suiteName=$(basename "$suite")
    local ginkgoTest="$suite/$suiteName.test"
    while IFS= read -r testcase _; do
      cmd="$ginkgoTest -ginkgo.v -ginkgo.focus \"$testcase\" $suite"
      ginkgoCmds="$ginkgoCmds\n$cmd"
    done < <($ginkgoTest -ginkgo.v -ginkgo.noColor -ginkgo.dryRun $suite | grep 'alb-test-case' | sed -e 's/.*alb-test-case\s*//g')
  done
  printf "$ginkgoCmds"
}

function alb-debug-e2e-test() {
  # not yet
  xdg-open 'vscode://fabiospampinato.vscode-debug-launcher/launch?args={"type":"go","name":"ginkgo","request":"launch","mode","exec","program":"./test/e2e/gateway/gateway.test","args":["-ginkgo.v", "-ginkgo.focus", "allowedRoutes should ok", "./test/e2e/gateway"]}'
}

function alb-run-e2e-test-one() {
  export DEV_MODE=true
  local cmd=$(alb-list-e2e-testcase | fzf)
  echo $cmd
  if [[ "$FAKE_HISTORY" == "true" ]]; then
    local suite=$(echo $cmd | awk '{print $(NF)}')
    local name=$(echo $cmd | awk '{print $(NF-1)}')
    add-history "ginkgo -v -focus "$name" $suite"
  fi
  eval $cmd
}

function alb-run-all-e2e-test() {
  local concurrent=${1:-4}
  local filter=${2:-""}

  alb-build-e2e-test
  echo concurrent is $concurrent
  local cmds=$(alb-list-e2e-testcase | grep "$filter")
  echo all-test "$(printf "$cmds" | wc -l)"

  echo "$cmds" >./cmds.cfg
  cat ./cmds.cfg
  if [[ "$concurrent" == "1" ]]; then
    export DEV_MODE="true"
    bash -x -e ./cmds.cfg 2>&1 | tee ./test.log
    return
  fi
  unset DEV_MODE
  unset KUBECONFIG
  local start=$(date +"%Y %m %e %T.%6N")
  cat ./cmds.cfg | tr '\n' '\0' | xargs -0 -P $concurrent -I{} bash -x -e -c '{} || exit 255 ' 2>&1 | tee ./test.log
  local end=$(date +"%Y %m %e %T.%6N")
  echo $start $end
  if cat ./test.log | grep '1 Failed'; then
    echo "sth wrong"
    return 1
  fi
}

function alb-go-coverage {
  local filter=${1:-""}
  # TODO it shoult include e2e test
  # translate from https://github.com/ory/go-acc
  rm -rf ./coverage*
  echo 'mode: atomic' >coverage.txt
  local coverpkg_list=$(go list ./... | grep -v e2e | grep -v pkg | grep -v migrate | sort | grep "$filter")
  local coverpkg=$(echo "$coverpkg_list" | tr "\n" ",")

  local fail="0"
  echo "$coverpkg"
  while IFS= read -r pkg; do
    echo "pkg $pkg"
    if [ -f ./coverage.tmp ]; then rm ./coverage.tmp; fi
    touch ./coverage.tmp
    go test -race -covermode=atomic -coverprofile=coverage.tmp -coverpkg "$coverpkg" $pkg
    fail="$?"
    echo "pkg test over $pkg $fail"
    if [[ ! "$fail" == "0" ]]; then
      break
    fi
    tail -n +2 ./coverage.tmp >>./coverage.txt
  done <<<"$coverpkg_list"

  if [[ ! "$fail" == "0" ]]; then
    return 1
  fi

  go tool cover -html=./coverage.txt -o coverage.html
  go tool cover -func=./coverage.txt >./coverage.report
  local total=$(grep total ./coverage.report | awk '{print $3}')
  echo $total
}

function go-unit-test {
  if [ -d ./alb-nginx/t/servroot ]; then
    rm -rf ./alb-nginx/t/servroot || true
  fi
  go test -v -coverprofile=coverage-all.out $(go list ./... | grep -v e2e)
  #   alb-go-coverage
}

function alb-envtest-install() {
  curl --progress-bar -sSLo envtest-bins.tar.gz "https://go.kubebuilder.io/test-tools/1.21.2/$(go env GOOS)/$(go env GOARCH)"
  mkdir -p /usr/local/kubebuilder
  tar -C /usr/local/kubebuilder --strip-components=1 -zvxf envtest-bins.tar.gz
  rm envtest-bins.tar.gz
  ls /usr/local/kubebuilder
  /usr/local/kubebuilder/bin/kube-apiserver --version
}

function alb-install-golang-test-dependency {
  wget https://dl.k8s.io/v1.24.1/kubernetes-client-linux-amd64.tar.gz && tar -zxvf kubernetes-client-linux-amd64.tar.gz && chmod +x ./kubernetes/client/bin/kubectl && mv ./kubernetes/client/bin/kubectl /usr/local/bin/kubectl && rm -rf ./kubernetes && rm ./kubernetes-client-linux-amd64.tar.gz
  #   kubectl version
  which kubectl
  echo "install helm"
  wget https://mirrors.huaweicloud.com/helm/v3.9.3/helm-v3.9.3-linux-amd64.tar.gz && tar -zxvf helm-v3.9.3-linux-amd64.tar.gz && chmod +x ./linux-amd64/helm && mv ./linux-amd64/helm /usr/local/bin/helm && rm -rf ./linux-amd64 && rm ./helm-v3.9.3-linux-amd64.tar.gz
  helm version
  apk update && apk add python3 py3-pip curl git build-base jq iproute2 openssl
  pip install crossplane -i https://mirrors.aliyun.com/pypi/simple
  alb-envtest-install
  git config --global --add safe.directory $PWD
  go version
  go env -w GO111MODULE=on
  go env -w GOPROXY=https://goproxy.cn,direct
  cd /tmp
  go install -v mvdan.cc/sh/v3/cmd/shfmt@latest
  go install -v github.com/onsi/ginkgo/ginkgo@latest
  cd -
  export GOFLAGS=-buildvcs=false
}

function alb-test-all-in-ci-golang {
  # base image build-harbor.alauda.cn/ops/golang:1.18-alpine3.15
  set -e # exit on err
  echo alb is $ALB
  echo pwd is $(pwd)
  local start=$(date +"%Y %m %e %T.%6N")
  alb-install-golang-test-dependency
  local end_install=$(date +"%Y %m %e %T.%6N")
  alb-lint-bash
  alb-lint-go
  local end_lint=$(date +"%Y %m %e %T.%6N")
  go-unit-test
  local end_unit_test=$(date +"%Y %m %e %T.%6N")
  echo "unit-test ok"
  which ginkgo
  echo $?
  alb-run-all-e2e-test
  echo $?
  local end_e2e=$(date +"%Y %m %e %T.%6N")
  echo "start" $start
  echo "install" $end_install
  echo "lint" $end_lint
  echo "unit-test" $end_unit_test
  echo "ginkgo" $end_ginkgo
  echo "e2e" $end_e2e
}

function alb-install-nginx-test-dependency {
  apk update && apk add luarocks luacheck lua perl-app-cpanminus wget curl make build-base perl-dev git neovim bash yq jq tree fd openssl
  cpanm --mirror-only --mirror https://mirrors.tuna.tsinghua.edu.cn/CPAN/ -v --notest Test::Nginx IPC::Run
}

function alb-test-all-in-ci-nginx {
  # base image build-harbor.alauda.cn/3rdparty/alb-nginx:v3.9-57-gb40a7de
  set -e # exit on err
  echo alb is $ALB
  echo pwd is $(pwd)
  local start=$(date +"%Y %m %e %T.%6N")
  if [ -z "$SKIP_INSTALL_NGINX_TEST_DEP" ]; then
    alb-install-nginx-test-dependency
  fi
  local end_install=$(date +"%Y %m %e %T.%6N")
  source ./alb-nginx/actions/common.sh
  #   alb-lint-lua # TODO
  local end_check=$(date +"%Y %m %e %T.%6N")
  test-nginx-in-ci
  local end_test=$(date +"%Y %m %e %T.%6N")
  echo "start " $start
  echo "install " $end_install
  echo "check" $end_check
  echo "test" $end_test
}
