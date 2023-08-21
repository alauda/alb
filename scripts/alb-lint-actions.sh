#!/bin/bash

function alb-lint-all() {
  alb-lint-bash
  echo "bash ok"
  alb-lint-go
  echo "go ok"
  #   alb-lint-lua
  #   echo "lua ok"
}

function alb-lint-bash() {
  # add "shellformat.flag": "-i 2", into vscode settings.json
  shfmt -i 2 -d ./scripts
  shfmt -i 2 -d ./template/actions
}

function alb-lint-bash-fix() {
  shfmt -i 2 -w ./scripts
  shfmt -i 2 -w ./template/actions
}

function alb-lint-go() {
  if [ ! "$(gofmt -l $(find . -type f -name '*.go' | grep -v ".deepcopy"))" = "" ]; then
    echo "go fmt check fail"
    return 1
  fi
  alb-lint-go-build
  alb-list-kind-e2e
}

function alb-lint-go-build() {
  go build -v -v -o ./bin/alb alauda.io/alb2/cmd/alb
  go build -v -v -o ./bin/alb alauda.io/alb2/cmd/operator
  echo "build alb ok"
  alb-build-e2e-test
  echo "build e2e ok"
  go test ./... -list=.
  echo "build test ok"
}

function alb-lint-go-fix {
  gofmt -w .
}

function alb-lint-lua() {
  # TODO add all lua file
  TOUCHED_LUA_FILE=("utils/common.lua" "worker.lua" "upstream.lua" "l7_redirect.lua" "cors.lua" "cert.lua")
  # shellcheck disable=SC2068
  for f in ${TOUCHED_LUA_FILE[@]}; do
    echo check format of $f
    local lua=./template/nginx/lua/$f
    lua-format --check -v $lua
    echo "format ok"
    luacheck ./$lua
  done
}

function alb-lint-fix {
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

check-branch-name
source ./scripts/alb-dev-actions.sh
alb-lint-all
EOF
  echo "$PREPUSH" >./.git/hooks/pre-push
  chmod a+x ./.git/hooks/pre-push
}
