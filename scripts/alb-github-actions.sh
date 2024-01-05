#!/bin/bash
function alb-github-gen-version() {
  local branch=$(echo "$GITHUB_HEAD_REF" | sed 's|/|-|g')
  echo "v$CURRENT_VERSION-$branch.$GITHUB_RUN_NUMBER.$GITHUB_RUN_ATTEMPT"
}

function alb-github-sync() {
  docker load <./alb/alb.tar
  docker load <./alb-nginx/alb-nginx.tar
  kind-load-image-in-current theseedoaa/alb-nginx:$(yq .global.images.alb2.tag ./alauda-alb2/values.yaml)
  kind-load-image-in-current theseedoaa/alb:$(yq .global.images.alb2.tag ./alauda-alb2/values.yaml)
}

function alb-build-github-chart() {
  #   alb-build-github-chart $RELEASE_TAG ./chart/alb ./xx
  local repo=$1
  local version=$2
  local chart_dir=$3
  local out_dir=$4

  local branch=$GITHUB_HEAD_REF
  local commit=$GITHUB_SHA
  cp ./.github/chart/alb/values.yaml $chart_dir
  cp ./.github/chart/alb/crds/crd.alauda.io_alaudaloadbalancer2.yaml $chart_dir/crds # do not served v1
  yq -i e ".global.images.alb2.tag |= \"$version\"" $chart_dir/values.yaml
  yq -i e ".global.registry.address |= \"$repo\"" $chart_dir/values.yaml
  yq -i e ".global.images.nginx.tag |= \"$version\"" $chart_dir/values.yaml
  yq -i e ".annotations.branch |= \"$branch\"" $chart_dir/Chart.yaml
  yq -i e ".annotations.commit |= \"$commit\"" $chart_dir/Chart.yaml
  yq -i e ".version |= \"$version\"" $chart_dir/Chart.yaml

  helm package --debug $chart_dir >/dev/null
  mv ./alauda-alb2-$version.tgz $out_dir
  echo "$out_dir/alauda-alb2-$version.tgz"
  return
}
