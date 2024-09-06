#!/bin/bash

function alb-lint-all() (
  set -e
  alb-lint-bash
  echo "bash ok"
  alb-lint-go
  echo "go ok"
  alb-lint-lua
  echo "lua ok"
  # golangci-lint oom in ci
  golangci-lint -v run -c ./.golangci.yml
)

function alb-lint-golangci() {
  golangci-lint -v run -c ./.golangci.yml
}

function alb-lint-in-ci() {
  alb-lint-bash
  echo "bash ok"
  alb-lint-go
  echo "go ok"
}

function alb-lint-bash() {
  # add "shellformat.flag": "-i 2", into vscode settings.json
  shfmt -i 2 -d ./scripts
  shfmt -i 2 -d ./template/actions
  shfmt -i 2 -d ./migrate/checklist
}

function alb-lint-bash-fix() {
  shfmt -i 2 -w ./scripts
  shfmt -i 2 -w ./template/actions
  shfmt -i 2 -w ./migrate/checklist
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

function alb-lint-lua-install() {
  # https://github.com/Koihik/LuaFormatter
  # use VS Marketplace Link: https://marketplace.visualstudio.com/items?itemName=Koihik.vscode-lua-format
  sudo luarocks install --server=https://luarocks.org/dev luaformatter #lua-format
  sudo luarocks install luacheck
}

function alb-lint-lua-need-format() {
  local f=$1
  if (head $f -n 10 | grep format:on); then
    echo "true"
  fi
  echo "false"
}

function alb-lint-lua() (
  while read -r f; do
    luacheck $f
    if [[ $? -ne 0 ]]; then
      echo "need fix $f"
      exit 1
      return
    fi
  done < <(alb-lua-list-all-app-file)

  # before i find a beeter way to format lua, disable this
  # the recommended way is to use lus-ls https://github.com/LuaLS/lua-language-server in vscode
  return
  #   # TODO add all lua file
  #   while read -r f; do
  #     lua-format --check -v $f
  #     if [[ $? -ne 0 ]]; then
  #       echo "need format $f"
  #       exit 1
  #       return
  #     fi
  #   done < <(alb-lua-list-all-needformat-file)

  echo "lua ok"
)

function alb-lua-lint-format-fix() (
  # shellcheck disable=SC2068
  while read -r f; do
    lua-format -i -v $f
  done < <(alb-lua-list-all-needformat-file)
)

function alb-lua-list-all-file() {
  alb-lua-list-all-test-file
  alb-lua-list-all-app-file
}

function alb-lua-list-all-test-file() {
  find $PWD/template/t -type f | grep '\.lua'
}

function alb-lua-list-all-app-file() {
  find $PWD/template/nginx/lua -type f | grep '\.lua' | grep -v 'types.lua' | grep -v 'vendor' | grep -v 'lua/resty/'
}

function alb-lua-list-all-needformat-file() {
  # TODO install luaformatter in ci
  if [[ -z $(which lua-format) ]]; then
    # echo "lua-format not installed"
    return
  fi
  while read -r f; do
    if [[ "false" == "$(alb-lint-lua-need-format $f)" ]]; then
      continue
    fi
    echo $f
  done < <(alb-lua-list-all-file)
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
