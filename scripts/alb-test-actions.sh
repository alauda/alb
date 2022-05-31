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

function alb-test-all-in-ci {
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