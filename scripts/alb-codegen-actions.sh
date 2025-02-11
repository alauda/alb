#!/bin/bash

function alb-crd-install-bin() (
  if [[ -z "$(which controller-gen)" ]]; then
    echo "controller-gen not found,  install it first"
    go install sigs.k8s.io/controller-tools/cmd/controller-gen@latest
  fi
  local generator_grous_path=$GOPATH/pkg/mod/k8s.io/code-generator@v0.24.4-rc.0/generate-groups.sh
  if [[ ! -f $generator_grous_path ]]; then
    go install k8s.io/code-generator@v0.24.4-rc.0
    chmod a+x $generator_grous_path
  fi
)

function alb-crd-gen() (
  set -ex
  alb-crd-install-bin

  rm -rf pkg/client
  rm -rf ./code_gen || true
  mkdir ./code_gen

  alb-crd-gen-crd
  alb-crd-gen-cli
  alb-crd-gen-deepcopy

  tree $ALB/code_gen
  cp -r $ALB/code_gen/alauda.io/alb2/pkg/* $ALB/pkg
  cp -r $ALB/code_gen/alauda.io/alb2/pkg/controller/* $ALB/pkg/controller
  rm -rf $ALB/code_gen

  # we should use --plural-exceptions "ALB2:alaudaloadbalancer2,but we could not. is too late. i just give up.
  find ./pkg/client -name '*.go' | xargs -i{} sed -i 's/Resource("alb2s")/Resource("alaudaloadbalancer2")/g' {}
  # 增加一个v2版本，v2版本和v2beta1保持一致，storageversion也用v2beta1.. 这样的好处是直接用kubectl get alb2 时能拿到正确的版本(v2) # (也不用改代码了)
  yq -i '.spec.versions += .spec.versions.1' ./deploy/chart/alb/crds/crd.alauda.io_alaudaloadbalancer2.yaml
  yq -i '.spec.versions.2.name = "v2"' ./deploy/chart/alb/crds/crd.alauda.io_alaudaloadbalancer2.yaml
  yq -i '.spec.versions.2.storage = false' ./deploy/chart/alb/crds/crd.alauda.io_alaudaloadbalancer2.yaml

  echo "gen ok"
)

function alb-crd-gen-crd() (
  # alb crd
  controller-gen crd paths=./pkg/apis/alauda/... output:crd:dir=./deploy/chart/alb/crds/
  # gateway crd
  # gateway的crd是直接复制的
)

function _generator_group() (
  $GOPATH/pkg/mod/k8s.io/code-generator@v0.24.4-rc.0/generate-groups.sh $@
)

function alb-crd-gen-cli() (
  local alb=$CUR_ALB_BASE
  _generator_group "client,lister,informer" \
    alauda.io/alb2/pkg/client alauda.io/alb2/pkg/apis \
    "alauda:gateway/v1alpha1 alauda:v1,v2beta1" \
    --go-header-file $alb/scripts/boilerplate.go.txt \
    --output-base "$alb/code_gen"
)

function alb-crd-gen-deepcopy() (
  local alb=$CUR_ALB_BASE
  set -x
  local GOBIN="$(go env GOBIN)"
  local gobin="${GOBIN:-$(go env GOPATH)/bin}"

  local ext_pkgs=$(go list $CUR_ALB_BASE/... | grep "alb2/pkg/controller/ext" | grep types | sort | uniq | awk 'ORS=","')
  local pkgs="alauda.io/alb2/pkg/apis/alauda/v1,alauda.io/alb2/pkg/apis/alauda/v2beta1,$ext_pkgs"
  "${gobin}/deepcopy-gen" $pkgs --input-dirs $pkgs -O zz_generated.deepcopy --go-header-file ./scripts/boilerplate.go.txt --output-base "$alb/code_gen"
)

function alb-gen-depgraph() (
  # it will take about 2 minutes
  goda graph "alauda.io/alb2/... - alauda.io/alb2/utils/... - alauda.io/alb2/pkg/utils/... - alauda.io/alb2/pkg/apis/... - alauda.io/alb2/pkg/client/...  - alauda.io/alb2/migrate/...  - alauda.io/alb2/test/..." >./alb.dep
  cat ./alb.dep | dot -Tsvg -o graph.svg
)

function alb-gen-mapping() (
  while read -r file; do
    echo "rm $file"
    rm "$file"
  done < <(find ./ -name "codegen_mapping_*" -type f)
  go run ./cmd/utils/map_gen/main.go
  while read -r file; do
    echo "fmt $file"
    go fmt "$file"
    gofumpt -w "$file"
  done < <(find ./ -name "codegen_mapping_*" -type f)
)
