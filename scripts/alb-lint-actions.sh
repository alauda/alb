#!/bin/bash

function alb-lint-all() (
  set -e -o pipefail
  cd $CUR_ALB_BASE
  echo "alb-lint-all"
  alb-lint-cspell
  echo "lint cspell ok"
  alb-lint-bash
  echo "bash ok"
  alb-lint-go
  echo "go ok"
  alb-lint-lua
  echo "lua ok"
  # golangci-lint oom in ci
  golangci-lint -v run -c ./.golangci.yml
)

function alb-lint-cspell() (
  cspell lint "./**/*.go" -c ./.cspell.json
  cspell lint "./**/*.md" --exclude "./template/nginx/lua/vendor/*" -c ./.cspell.json
  cspell lint "./**/*.lua" --exclude "./template/nginx/lua/vendor/*" -c ./.cspell.json
)

function alb-lint-golangci() {
  golangci-lint -v run -c ./.golangci.yml
}

function alb-lint-in-ci() {
  alb-lint-cspell
  echo "spell ok"
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
  # shellcheck disable=SC2046
  if [ ! "$(gofmt -l $(find . -type f -name '*.go' | grep -v ".deepcopy"))" = "" ]; then
    echo "go fmt check fail"
    return 1
  fi
  alb-lint-go-build
  alb-list-kind-e2e
}

function alb-lint-gofumpt() {
  gofumpt -l ./
}

function alb-lint-gofumpt-fix() {
  alb-lint-gofumpt | xargs gofumpt -w
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
  luarocks install luacheck
  rm ./linux-x64.tar.gz || true
  wget https://github.com/CppCXY/EmmyLuaCodeStyle/releases/download/1.5.6/linux-x64.tar.gz
  tar -xvf linux-x64.tar.gz
  mv ./linux-x64/bin/CodeFormat /usr/bin/CodeFormat
}

function alb-lint-lua-need-format() {
  local f=$1
  if (head $f -n 10 | grep format:on); then
    echo "true"
  fi
  echo "false"
}

function alb-lint-lua() (
  alb-lint-lua-luacheck
  alb-lint-lua-emmy
)

function alb-lint-lua-luacheck() {
  while read -r f; do
    luacheck $f
    if [[ $? -ne 0 ]]; then
      echo "need fix $f"
      exit 1
      return
    fi
  done < <(alb-lua-list-all-app-file)
}

function alb-lint-lua-emmy-all() {
  CodeFormat check -w . -c ./.editorconfig -ig "vendor/*;" 2>&1 | grep Check
}

function alb-lint-lua-emmy() {
  while read -r f; do
    if head -n 1 "$f" | grep 'format:on' | grep 'style:emmy'; then
      alb-lint-lua-emmy-format-check $f
    fi
  done < <(alb-lua-list-all-file)
}

function alb-lint-lua-emmy-format-check() {
  local f=$1
  CodeFormat check -f $f -c ./.editorconfig
  return
}

function alb-lint-lua-emmy-format-install-arch() (
  yay -S code-format-bin
)

function alb-lint-lua-emmy-format-fix() (
  return
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
