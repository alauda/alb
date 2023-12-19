#!/bin/bash
# shellcheck disable=SC2120,SC2155,SC2181

function alb-debug-e2e-test() {
  # not yet
  xdg-open 'vscode://fabiospampinato.vscode-debug-launcher/launch?args={"type":"go","name":"ginkgo","request":"launch","mode","exec","program":"./test/e2e/gateway/gateway.test","args":["-ginkgo.v", "-ginkgo.focus", "allowedRoutes should ok", "./test/e2e/gateway"]}'
}

function alb-build-e2e-test() {
  ginkgo -dryRun -v ./test/e2e
}

function alb-go-test-all-with-coverage() {
  rm ./coverage.txt || true
  alb-go-unit-test
  local end_unit=$(date +"%Y %m %e %T.%6N")
  alb-run-all-e2e-test
  local end_e2e=$(date +"%Y %m %e %T.%6N")
  echo "end_unit $end_unit"
  echo "end_e2e $end_e2e"

  tail -n +2 ./test/e2e/coverage.e2e >>./coverage.txt

  sed -e '1i\mode: atomic' ./coverage.txt >./coverage.txt.all
  mv ./coverage.txt.all ./coverage.txt
  go tool cover -html=./coverage.txt -o coverage.html
  go tool cover -func=./coverage.txt >./coverage.report
  local total=$(grep total ./coverage.report | awk '{print $3}')
  echo $total
}

function alb-run-all-e2e-test() (
  set -e
  # TODO 覆盖率
  local concurrent=${1:-1}
  local filter=${2:-""}
  echo concurrent $concurrent filter $filter
  if [[ "$filter" != "" ]]; then
    ginkgo -failFast -focus "$filter" ./test/e2e
    return
  fi
  if [[ "$concurrent" == "1" ]]; then
    local all=$(ginkgo -dryRun -v ./test/e2e | grep alb-test-case | wc -l)
    local i=0
    while read tcase; do
      tcase=$(echo $tcase | xargs)
      echo "run case $tcase"
      echo "$tcase $i/$all" >./.current-test
      ginkgo -failFast -focus "$tcase" ./test/e2e
      i=$((i + 1))
    done < <(ginkgo -dryRun -noColor -v ./test/e2e | grep alb-test-case | sed 's/alb-test-case//g' | sort)
    return
  fi

  local coverpkg_list=$(go list ./... | grep -v e2e | grep -v test | grep -v "/pkg/client" | grep -v migrate | sort | uniq | grep "$filter")
  local coverpkg=$(echo "$coverpkg_list" | tr "\n" ",")
  ginkgo -v -cover -covermode=atomic -coverpkg="$coverpkg" -coverprofile=coverage.e2e -failFast -p -nodes $concurrent ./test/e2e
)

function alb-go-unit-test() {
  local filter=${1:-""}
  # TODO it shoult include e2e test
  # translate from https://github.com/ory/go-acc
  local coverpkg_list=$(go list ./... | grep -v e2e | grep -v test | grep -v "/pkg/client" | grep -v migrate | sort | uniq | grep "$filter")
  local coverpkg=$(echo "$coverpkg_list" | tr "\n" ",")

  local fail="0"
  echo "$coverpkg"
  while IFS= read -r pkg; do
    echo "pkg $pkg"
    if [ -f ./coverage.tmp ]; then rm ./coverage.tmp; fi
    touch ./coverage.tmp
    go test -v -race -covermode=atomic -coverprofile=coverage.tmp -coverpkg "$coverpkg" $pkg
    fail="$?"
    echo "pkg test over $pkg $fail"
    if [[ ! "$fail" == "0" ]]; then
      break
    fi
    tail -n +2 ./coverage.tmp >>./coverage.txt
  done <<<"$coverpkg_list"

  if [[ ! "$fail" == "0" ]]; then
    echo "unit test wrong"
    return 1
  fi
}

function alb-envtest-install() {
  # TODO use http://prod-minio.alauda.cn/acp/
  curl --progress-bar -sSLo envtest-bins.tar.gz "https://go.kubebuilder.io/test-tools/1.24.2/$(go env GOOS)/$(go env GOARCH)"
  #   curl --progress-bar -sSLo envtest-bins.tar.gz "http://prod-minio.alauda.cn:80/acp/envtest-bins.1.24.2.tar.gz"
  mkdir -p /usr/local/kubebuilder
  tar -C /usr/local/kubebuilder --strip-components=1 -zvxf envtest-bins.tar.gz
  rm envtest-bins.tar.gz
  ls /usr/local/kubebuilder
  /usr/local/kubebuilder/bin/kube-apiserver --version
}

function alb-install-golang-test-dependency() {
  ls
  which helm || true
  if [ -f "$(which helm)" ]; then echo "dependency already installed" return; else echo "dependency not installed. install it"; fi

  rm kubernetes-client-linux-amd64.tar.gz || true
  wget https://dl.k8s.io/v1.24.1/kubernetes-client-linux-amd64.tar.gz && tar -zxvf kubernetes-client-linux-amd64.tar.gz && chmod +x ./kubernetes/client/bin/kubectl && mv ./kubernetes/client/bin/kubectl /usr/local/bin/kubectl && rm -rf ./kubernetes && rm ./kubernetes-client-linux-amd64.tar.gz
  wget http://prod-minio.alauda.cn/acp/kubectl-v1.24.1 && chmod +x ./kubectl-v1.24.1 && mv ./kubectl-v1.24.1 /usr/local/bin/kubectl
  which kubectl

  echo "install helm"
  #   rm helm-v3.9.3-linux-amd64.tar.gz || true
  wget https://mirrors.huaweicloud.com/helm/v3.9.3/helm-v3.9.3-linux-amd64.tar.gz && tar -zxvf helm-v3.9.3-linux-amd64.tar.gz && chmod +x ./linux-amd64/helm && mv ./linux-amd64/helm /usr/local/bin/helm && rm -rf ./linux-amd64 && rm ./helm-v3.9.3-linux-amd64.tar.gz
  #   wget http://prod-minio.alauda.cn/acp/helm-v3.9.3 && chmod +x ./helm-v3.9.3 && mv ./helm-v3.9.3 /usr/local/bin/helm

  helm version

  apk update && apk add python3 py3-pip curl git build-base jq iproute2 openssl tree
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

function alb-test-all-in-ci-golang() {
  set -e # exit on err
  #   set -x
  echo alb is $ALB
  echo pwd is $(pwd)
  export ALB_ROOT=$(pwd)
  local start=$(date +"%Y %m %e %T.%6N")
  alb-install-golang-test-dependency
  local end_install=$(date +"%Y %m %e %T.%6N")
  tree ./
  alb-lint-all
  local end_lint=$(date +"%Y %m %e %T.%6N")
  alb-go-test-all-with-coverage
  local end_test=$(date +"%Y %m %e %T.%6N")

  echo "$start"
  echo "$end_install"
  echo "$end_lint"
  echo "$end_test"
}

function alb-list-kind-e2e() {
  ginkgo -debug -v -dryRun ./test/kind/e2e
}

function alb-list-e2e() {
  ginkgo -dryRun -noColor -v ./test/e2e | grep alb-test-case | sed 's/alb-test-case//g' | sort
}

function alb-debug-e2e() {
  alb-run-all-e2e-test | tee ./test.log
  cat ./test.log | grep 'test-case' | rg -o '.*alb-test-case([^:]*):' -r '$1' | xargs -I{} echo {} | uniq | sort >./run.test
  alb-list-e2e | xargs -I {} echo {} >all.test
  diff ./run.test ./all.test
}

function alb-test-kind() {
  ginkgo -debug -v -dryRun ./test/kind/e2e
}
