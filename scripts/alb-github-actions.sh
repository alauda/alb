#!/bin/bash

function alb-gh-install-from-release() (
  local ver=$1
  local image=$(helm install alb-operator alb/alauda-alb2 --version $ver --dry-run | grep image: | awk '{print $2}' | tr -d '"')
  kind-load-image-in-current $image
  helm install alb-operator alb/alauda-alb2 --version $ver
  return
)

function alb-gh-install-from-artifact() {
  local runid=$1

  if [ -z "$runid" ]; then
    echo "runid is required"
    return
  fi
  rm -rf gh-debug
  gh run download "$runid" -D gh-debug

  docker load <./gh-debug/alb/alb.tar
  (
    cd ./gh-debug/alb-chart
    tar -xvzf ./alauda-alb2.tgz
  )
  kind-load-image-in-current theseedoaa/alb:$(yq4 eval .global.images.alb2.tag ./gh-debug/alb-chart/alauda-alb2/values.yaml)
  helm install alb-operator ./gh-debug/alb-chart/alauda-alb2
  # do some test
}

function kind-load-image-in-current() (
  local cluster=$(kind get clusters | head -n1)
  local image=$1
  docker pull $image
  kind load docker-image $image --name $cluster
)

function alb-gh-wk-build-and-relase-nginx() (
  local branch=${1-$(git rev-parse --abbrev-ref HEAD)}
  local relase=${2-false}
  gh workflow run build-openresty.yaml --ref $branch -f do_release=$relase
)

function alb-gh-wk-build-and-relase-alb() (
  local branch=${1-$(git rev-parse --abbrev-ref HEAD)}
  local relase=${2-false}
  local skip_test=${3-false}
  gh workflow run build.yaml --ref $branch -f do_release=$relase -f skip_test=$skip_test
)
