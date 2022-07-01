#!/bin/bash

function alb-build-e2e-test {
  local base=$(pwd)
  local ginkgoCmds=""
  for suite_test in $(find ./test/e2e -name suite_test.go); do
    local suite=$(dirname "$suite_test")
    local suiteName=$(basename "$suite")
    local ginkgoTest="$suite/$suiteName.test"
    ginkgo build $suite 
  done

}

function alb-run-all-e2e-test {
  export ALB_IGNORE_FOCUS="true"
  local base=$(pwd)
  local ginkgoCmds=""
  for suite_test in $(find ./test/e2e -name suite_test.go); do
    local suite=$(dirname "$suite_test")
    local suiteName=$(basename "$suite")
    local ginkgoTest="$suite/$suiteName.test"
    rm -rf $suite/viper-config.toml
    cp $base/viper-config.toml $suite

    ginkgo build $suite 
    file $ginkgoTest
    if [ $? != 0 ]; then
      echo "build $suite failed"
      exit 1
    fi
    while IFS= read -r testcase _; do
      cmd="$ginkgoTest -ginkgo.v -ginkgo.focus \"$testcase\" $suite"
      ginkgoCmds="$ginkgoCmds\n$cmd"
    done < <($ginkgoTest -ginkgo.v -ginkgo.noColor -ginkgo.dryRun $suite | grep 'alb-test-case' | sed -e 's/.*alb-test-case\s*//g')
  done
  shopt -s xpg_echo
  touch $HOME/.ack-ginkgo-rc
  echo "$ginkgoCmds" > cmds.cfg
  parallel -j 4 < cmds.cfg
}


TOUCHED_LUA_FILE=("utils/common.lua" "worker.lua" "upstream.lua" "l7_redirect.lua" "cors.lua" "rewrite_response.lua" "cert.lua")
function alb-lua-format-check {
  # shellcheck disable=SC2068
  for f in ${TOUCHED_LUA_FILE[@]}; do
    echo check format of $f
    local lua=./template/nginx/lua/$f
    lua-format --check -v $lua
  done
}

function alb-lua-format-format {
  # shellcheck disable=SC2068
  for f in ${TOUCHED_LUA_FILE[@]}; do
    echo format $f
    local lua=./template/nginx/lua/$f
    lua-format -i -v $lua
  done
}

function alb-init-git-hook {
  read -r -d "" PREPUSH <<EOF
#!/bin/bash
set -e

function check-branch-name {
    current_branch=\$(git branch --show-current |tr -d '\n\r')
    if [[ \$current_branch == *acp* ]] ; 
    then
        echo "let's use ACP.."
        exit -1
    fi
}

sudo rm -rf ./alb-nginx/t/servroot # T_T
check-branch-name
make test
make all-e2e-envtest
make lua-test
cd chart
helm lint -f ./values.yaml
EOF
  echo "$PREPUSH" >./.git/hooks/pre-push
  chmod a+x ./.git/hooks/pre-push
}

function alb-go-coverage {
  # copy from https://github.com/ory/go-acc
  touch ./coverage.tmp
  echo 'mode: atomic' >coverage.txt
  go list ./... | grep -v /e2e | grep -v /pkg | xargs -n1 -I{} sh -c 'go test -race -covermode=atomic -coverprofile=coverage.tmp -coverpkg $(go list ./... | grep -v /pkg |grep -v /e2e | tr "\n" ",") {} && tail -n +2 coverage.tmp >> coverage.txt || exit 255' && rm coverage.tmp
  go tool cover -func=./coverage.txt
  go tool cover -html=./coverage.txt -o coverage.html
}

function go-fmt-fix {
  gofmt -w .
}

function go-lint {
  if [ ! "$(gofmt -l $(find . -type f -name '*.go' | grep -v ".deepcopy"))" = "" ]; then
    echo "go fmt check fail"
    exit 1
  fi
  go vet .../..
}

function go-unit-test {
  if [ -d ./alb-nginx/t/servroot ]; then
    rm -rf ./alb-nginx/t/servroot || true
  fi
  go test -v -coverprofile=coverage-all.out $(go list ./... | grep -v e2e)
}

function alb-install-golang-test-dependency {
  wget https://dl.k8s.io/v1.24.1/kubernetes-client-linux-amd64.tar.gz && tar -zxvf  kubernetes-client-linux-amd64.tar.gz && chmod +x ./kubernetes/client/bin/kubectl && mv ./kubernetes/client/bin/kubectl /usr/local/bin/kubectl && rm -rf ./kubernetes && rm ./kubernetes-client-linux-amd64.tar.gz
  apk update && apk add parallel python3 py3-pip curl git build-base jq iproute2 openssl
  pip install crossplane
  curl --progress-bar  -sSLo envtest-bins.tar.gz "https://go.kubebuilder.io/test-tools/1.19.2/$(go env GOOS)/$(go env GOARCH)" && \
    mkdir -p /usr/local/kubebuilder && \
    tar -C /usr/local/kubebuilder --strip-components=1 -zvxf envtest-bins.tar.gz && \
    rm envtest-bins.tar.gz && \
	ls /usr/local/kubebuilder 
}

function alb-test-all-in-ci-golang {
  # base image build-harbor.alauda.cn/ops/golang:1.18-alpine3.15
  set -e # exit on err
  echo alb is $ALB
  echo pwd is $(pwd)
  alb-install-golang-test-dependency
  git config --global --add safe.directory $PWD
  go version
  local START=$(date +%s)
  go env -w GO111MODULE=on
  go env -w GOPROXY=https://goproxy.cn,direct
  export GOFLAGS=-buildvcs=false
  go-lint
  go-unit-test
  echo "unit-test ok"
  go install github.com/onsi/ginkgo/ginkgo
  which ginkgo
  echo $?
  alb-run-all-e2e-test
}

function alb-install-nginx-test-dependency {
  apk update && apk add  luacheck lua  perl-app-cpanminus wget curl make build-base perl-dev git neovim bash yq jq tree fd openssl
  cpanm -v --notest Test::Nginx IPC::Run 
  cd / 
  git clone https://gitclone.com/github.com/ledgetech/lua-resty-http.git && cp lua-resty-http/lib/resty/* /usr/local/openresty/site/lualib/resty/
  cd -
#   wget -O lua-format https://github.com/Koihik/vscode-lua-format/raw/master/bin/linux/lua-format && chmod a+x ./lua-format &&  mv ./lua-format /usr/bin/ && lua-format --help
}

function alb-test-all-in-ci-nginx {
  # base image build-harbor.alauda.cn/3rdparty/alb-nginx:v3.9-57-gb40a7de
  set -e # exit on err
  echo alb is $ALB
  echo pwd is $(pwd)
  alb-install-nginx-test-dependency
  source ./alb-nginx/actions/common.sh
  luacheck ./template/nginx/lua
#   alb-lua-format-check
  test-nginx-in-ci
}


function alb-test-all-in-ci {
  # keep build env still work
  # the current dir in ci is sth like /home/xx/xx/acp-alb-test
  set -e # exit on err
  echo alb is $ALB
  echo pwd is $(pwd)
  source /root/.gvm/scripts/gvm
  gvm list
  gvm use go1.18 || true
  go version
  local START=$(date +%s)
  go env -w GO111MODULE=on
  go env -w GOPROXY=https://goproxy.cn,direct
  go-lint
  luacheck ./template/nginx/lua
  alb-lua-format-check
  go-unit-test
  echo "unit-test ok"
  go install github.com/onsi/ginkgo/ginkgo
  which ginkgo
  echo $?
  alb-run-all-e2e-test
  source ./alb-nginx/actions/common.sh
  test-nginx-in-ci
  local END=$(date +%s)
  echo "all-time: " $(echo "scale=3; $END - $START" | bc) "s"
}



function alb-list-e2e-testcase {
  for suite_test in $(find ./test/e2e -name suite_test.go); do
    local suite=$(dirname "$suite_test")
    ginkgo -v -noColor -dryRun $suite | grep 'alb-test-case'
  done
}