#!/bin/bash
# shellcheck disable=SC2120,SC2155,SC2181

function alb-debug-e2e-test() {
  # not yet
  xdg-open 'vscode://fabiospampinato.vscode-debug-launcher/launch?args={"type":"go","name":"ginkgo","request":"launch","mode","exec","program":"./test/e2e/gateway/gateway.test","args":["-ginkgo.v", "-ginkgo.focus", "allowedRoutes should ok", "./test/e2e/gateway"]}'
}

function alb-build-e2e-test() {
  ginkgo -dry-run ./test/e2e
}

function alb-go-test-all-with-coverage() {
  set -e
  echo "life: start test"
  env
  #   alb-run-checklist-test
  rm ./coverage.txt || true
  alb-go-unit-test
  local end_unit=$(date +"%Y %m %e %T.%6N")
  echo "life: unittest ok"
  alb-run-all-e2e-test
  local end_e2e=$(date +"%Y %m %e %T.%6N")
  echo "life: e2e ok"
  local end_checklist=$(date +"%Y %m %e %T.%6N")
  echo "end_unit $end_unit"
  echo "end_e2e $end_e2e"
  echo "end_checklist $end_checklist"

  tail -n +2 ./coverage.unit >>./coverage.txt
  tail -n +2 ./coverage.e2e >>./coverage.txt

  sed -e '1i\mode: atomic' ./coverage.txt >./coverage.txt.all
  cat ./coverage.txt.all | grep -v main.go | grep -v test_utils | grep -v "albctl/cmd" | grep -v "deepcopy.go" | grep -v "pkg/apis/alauda/" >./coverage.txt
  alb-go-coverage-gen ./coverage.txt
  alb-go-coverage-show ./coverage.txt
}

function alb-go-coverage-show() (
  local c=${1-"./coverage.txt"}
  go tool cover -func=$c | grep -v '100' | grep -v 'total:' | awk '{print $1, $2, substr($3, 1, length($3)-1)}' | sort -k3,3 -nr | column -t
)

function alb-go-coverage-gen() (
  local c=$1
  go tool cover -html=$c -o ./coverage.html
  go tool cover -func=$c >./coverage.report
  local total=$(grep total ./coverage.report | awk '{print $3}')
  echo $total
  return
)

function alb-run-checklist-test() (
  set -e -o pipefail
  echo "life: checklist start"
  ginkgo -v ./test/checklist
  echo "life: checklist end"
)

function alb-run-all-e2e-test() (
  set -e -o pipefail
  local concurrent=${1:-3}
  local filter=${2:-""}
  echo concurrent $concurrent filter $filter
  if [[ "$filter" != "" ]]; then
    ginkgo --fail-fast -focus "$filter" ./test/e2e | tee ./all.e2e.log
    return
  fi

  local coverpkg_list=$(go list ./... | grep -v e2e | grep -v test | grep -v "/pkg/client" | grep -v migrate | sort | uniq | grep "$filter")
  local coverpkg=$(echo "$coverpkg_list" | tr "\n" ",")
  unset DEV_MODE                          # dev_mode 会导致k8s只启动一个 无法并行测试。。
  rm ./test/e2e/ginkgo-node-*.log || true # clean old test log
  ginkgo -v -cover -covermode=atomic -coverpkg="$coverpkg" -coverprofile=coverage.e2e --fail-fast -p -nodes $concurrent ./test/e2e | tee ./all.e2e.log
  # for ci
  if [ -f ./debug ]; then
    while true; do
      echo "debug"
      sleep 1s
    done
  fi
)

function alb-go-build-unit-test() {
  go test -run -c alauda.io/alb2/... | grep -v 'ok' | grep -v 'no test files'
}

function alb-go-unit-test() (
  set -e -o pipefail
  local concurrent=${1:-3}
  local filter=${2:-""}
  local s=$(date)
  echo "s $s"
  # https://github.com/ory/go-acc
  local coverpkg_list=$(go list ./... | grep -v 'alb2/test/' | grep -v "/pkg/client" | grep -v migrate | sort | uniq | grep "$filter")
  local coverpkg=$(echo "$coverpkg_list" | tr "\n" ",")

  go test -p $concurrent -v -race -covermode=atomic -coverprofile=coverage.unit -coverpkg "$coverpkg" $(go list ./... | grep -v 'alb2/test/' | grep -v "/pkg/client" | grep -v migrate | sort | uniq | grep "$filter") | tee ./unit_test.log
  local e=$(date)
  echo $s $e
)

function alb-envtest-install() {
  curl --progress-bar -sSLo envtest-bins.tar.gz $(__at_resolve_url envtest)
  mkdir -p /usr/local/kubebuilder
  tar -C /usr/local/kubebuilder --strip-components=1 -zvxf envtest-bins.tar.gz
  rm envtest-bins.tar.gz
  ls /usr/local/kubebuilder
  /usr/local/kubebuilder/bin/kube-apiserver --version
}

function __at_resolve_url() {
  local name="$1"
  local arch=$(uname -m)

  local envtest_url="https://go.kubebuilder.io/test-tools/1.24.2/$(go env GOOS)/$(go env GOARCH)"
  local cfg=$(
    cat <<EOF
{
 "kubectl_x86_64_online": "https://dl.k8s.io/v1.24.1/kubernetes-client-linux-amd64.tar.gz",
 "helm_x86_64_online": "https://mirrors.huaweicloud.com/helm/v3.9.3/helm-v3.9.3-linux-amd64.tar.gz",
 "golangcli_x86_64_online": "https://github.com/golangci/golangci-lint/releases/download/v1.59.1/golangci-lint-1.59.1-illumos-amd64.tar.gz",
 "envtest_x86_64_online": "$envtest_url",

 "kubectl_arm_online": "https://get.helm.sh/helm-v3.7.0-linux-amd64.tar.gz",
 "helm_arm_online": "https://get.helm.sh/helm-v3.7.0-linux-amd64.tar.gz",
 "golangcli_arm_online": "https://get.helm.sh/helm-v3.7.0-linux-amd64.tar.gz",
 "envtest_arm_online": "$envtest_url",
 
 "kubectl_x86_64_offline": "http://prod-minio.alauda.cn/acp/ci/alb/build/kubernetes-client-linux-amd64.tar.gz",
 "helm_x86_64_offline": "http://prod-minio.alauda.cn/acp/ci/alb/build/helm-v3.9.3-linux-amd64.tar.gz",
 "golangcli_x86_64_offline": "http://prod-minio.alauda.cn/acp/ci/alb/build/golangci-lint-1.59.1-illumos-amd64.tar.gz",
 "envtest_x86_64_offline": "http://prod-minio.alauda.cn:80/acp/envtest-bins.1.24.2.tar.gz"
}
EOF
  )
  local mode="offline"
  if [[ "$ALB_ONLINE" == "true" ]]; then
    mode="online"
  fi
  local url=$(echo "$cfg" | jq -r ".${name}_${arch}_${mode}")
  if [[ -z "$url" ]]; then
    echo "not found $name $arch $mode"
    return 1
  fi
  echo $url
}

function alb-install-golang-test-dependency() {
  ls
  which helm || true
  if [ -f "$(which helm)" ]; then echo "dependency already installed" return; else echo "dependency not installed. install it"; fi

  # rm -rf kubernetes-client-linux-amd64.tar.gz &&  wget  &&
  local kubectl_url=$(__at_resolve_url kubectl)

  wget $kubectl_url
  tar -zxvf kubernetes-client-linux-amd64.tar.gz
  chmod +x ./kubernetes/client/bin/kubectl
  mv ./kubernetes/client/bin/kubectl /usr/local/bin/kubectl
  rm -rf ./kubernetes
  rm ./kubernetes-client-linux-amd64.tar.gz

  which kubectl

  echo "install helm"
  local helm_url=$(__at_resolve_url helm)
  wget $helm_url
  tar -zxvf helm-v3.9.3-linux-amd64.tar.gz && chmod +x ./linux-amd64/helm && mv ./linux-amd64/helm /usr/local/bin/helm && rm -rf ./linux-amd64 && rm ./helm-v3.9.3-linux-amd64.tar.gz

  helm version
  # url -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.55.2
  local golangci_lint=$(__at_resolve_url golangcli)
  wget $golangci_lint
  tar -zxvf ./golangci-lint-1.59.1-illumos-amd64.tar.gz
  chmod +x ./golangci-lint-1.59.1-illumos-amd64/golangci-lint && mv ./golangci-lint-1.59.1-illumos-amd64/golangci-lint /usr/local/bin/golangci-lint
  rm -rf ./golangci-lint-1.59.1-illumos-amd64.tar.gz
  rm -rf ./golangci-lint-1.59.1-illumos-amd64

  apk update && apk add shfmt ginkgo curl git build-base jq yq iproute2 openssl tree nodejs npm util-linux-misc
  alb-envtest-install
  git config --global --add safe.directory $PWD
  go version
  go env -w GO111MODULE=on
  go env -w GOPROXY=https://goproxy.io,direct
  cd /tmp
  echo "install cspell"
  npm install -g cspell@latest
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
  alb-lint-in-ci
  local end_lint=$(date +"%Y %m %e %T.%6N")
  alb-go-test-all-with-coverage
  local end_test=$(date +"%Y %m %e %T.%6N")

  echo "$start"
  echo "$end_install"
  echo "$end_lint"
  echo "$end_test"
}

function alb-list-kind-e2e() {
  ginkgo -v -dry-run ./test/kind/e2e
}

function alb-list-e2e() {
  ginkgo -dry-run --no-color -v ./test/e2e | grep alb-test-case | sed 's/alb-test-case//g' | sort
}

function alb-debug-e2e() {
  alb-run-all-e2e-test | tee ./test.log
  cat ./test.log | grep 'test-case' | rg -o '.*alb-test-case([^:]*):' -r '$1' | xargs -I{} echo {} | uniq | sort >./run.test
  alb-list-e2e | xargs -I {} echo {} >all.test
  diff ./run.test ./all.test
}

function alb-test-kind() {
  ginkgo -debug -v -dry-run ./test/kind/e2e
}
