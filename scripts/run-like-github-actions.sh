#!/bin/bash
set -e
export IMAGE_REPO="theseedoaa"
# required env $CURRENT_VERSION $RELEASE_ME

function alb-gh-test-alb-go() (
  pwd
  ls
  echo "test go"
  local go_build_base="docker.io/library/golang:$(alb-gh-get-gobuild-ver)"
  export ALB_ONLINE="true"
  ./scripts/run-like-ci-go.sh $go_build_base
)

function alb-gh-test-alb-nginx() (
  pwd
  ls
  echo "test nginx"
  local image=$(docker images | grep theseedoaa/alb | head -1 | awk '{printf "%s:%s",$1,$2}')
  echo "test nginx $image"
  ./scripts/run-like-ci-nginx.sh $image
)

function alb-gh-build-alb() (
  env
  local chart_ver=$(alb-github-gen-version)
  echo "version $CURRENT_VERSION this ver $chart_ver is_release $RELEASE_ME"

  local alb_nginx_base="$IMAGE_REPO/alb-nginx-base:$(alb-gh-get-nginx-ver)"
  local go_build_base="docker.io/library/golang:$(alb-gh-get-gobuild-ver)"
  #  build images
  docker buildx build \
    --network=host \
    --platform linux/amd64 \
    -t $IMAGE_REPO/alb:$chart_ver \
    --build-arg GO_BUILD_BASE=$go_build_base \
    --build-arg ALB_ONLINE=true \
    --build-arg OPENRESTY_BASE=$alb_nginx_base \
    -o type=docker \
    -f ./Dockerfile .
  docker images
  docker save $IMAGE_REPO/alb:$chart_ver >alb.tar
  ls
  # build chart
  rm -rf .cr-release-packages
  mkdir -p .cr-release-packages
  chart=$(alb-build-github-chart $IMAGE_REPO $chart_ver ./deploy/chart/alb .cr-release-packages/)
  cp $chart alauda-alb2.tgz
  tree ./deploy/chart/alb
  tree .cr-release-packages
  cat ./deploy/chart/alb/Chart.yaml
  return
)

function alb-gh-release-alb() (
  if [[ "$RELEASE_ME" != "true" ]]; then
    echo "skip release"
    return
  fi
  echo "in release"
  # push docker
  source ./scripts/alb-dev-actions.sh
  export VERSION=$(alb-github-gen-version)
  echo "$DOCKER_PASSWORD" | docker login -u "$DOCKER_USERNAME" --password-stdin
  docker push $IMAGE_REPO/alb:$VERSION

  # push chart
  owner=$(cut -d '/' -f 1 <<<"$GITHUB_REPOSITORY")
  repo=$(cut -d '/' -f 2 <<<"$GITHUB_REPOSITORY")

  args=(--owner "$owner" --repo "$repo" --charts-dir "./deploy/chart/alb" --skip-packaging "true" --pages-branch "gh-pages")

  echo "sync chart"
  git status
  git log | head -n 30
  git remote -v
  git remote update
  git branch -r

  .github/cr.sh "${args[@]}"
  return
)

function alb-gh-build-nginx() (
  local ver=$(alb-gh-get-nginx-ver)
  local RESTY_PCRE_VERSION=$(cat ./template/Dockerfile.openresty | grep RESTY_PCRE_VERSION= | awk -F = '{print $2}' | tr -d '"')
  local RESTY_PCRE_BASE="https://downloads.sourceforge.net/project/pcre/pcre/$RESTY_PCRE_VERSION/pcre-$RESTY_PCRE_VERSION.tar.gz"
  local resty_base="docker.io/library/alpine"
  docker buildx build \
    --progress=plain \
    --no-cache \
    --network=host \
    --platform linux/amd64 \
    -t $IMAGE_REPO/alb-nginx-base:$ver \
    --build-arg RESTY_IMAGE_BASE=$resty_base \
    --build-arg RESTY_PCRE_BASE=$RESTY_PCRE_BASE \
    -o type=docker \
    -f ./template/Dockerfile.openresty \
    ./
  docker images
  docker save $IMAGE_REPO/alb-nginx-base:$ver >alb-nginx-base.tar
  return
)

function alb-gh-release-nginx() (
  echo "$DOCKER_PASSWORD" | docker login -u "$DOCKER_USERNAME" --password-stdin
  docker images
  local ver=$(alb-gh-get-nginx-ver)
  docker push $IMAGE_REPO/alb-nginx-base:$ver
  return
)

function alb-gh-get-nginx-ver() (
  cat ./Dockerfile | grep OPENRESTY_BASE | grep alb-nginx | awk -F : '{print $2}'
)

function alb-gh-get-gobuild-ver() (
  cat ./Dockerfile | grep GO_BUILD_BASE= | awk -F : '{print $2}'
)

function alb-github-gen-version() {
  local branch=$(echo "${GITHUB_HEAD_REF:-${GITHUB_REF#refs/heads/}}" | sed 's|/|-|g')
  if [[ "$branch" == "master" ]]; then
    echo v$CURRENT_VERSION
    return
  fi
  echo "v$CURRENT_VERSION-$branch.$GITHUB_RUN_NUMBER.$GITHUB_RUN_ATTEMPT"
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
  cp ./deploy/chart/alb/crds/crd.alauda.io_alaudaloadbalancer2.yaml $chart_dir/crds # do not served v1
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

if [ "$0" = "$BASH_SOURCE" ]; then
  if [[ "$1" == "build-nginx" ]]; then
    alb-gh-build-nginx
  fi
  if [[ "$1" == "release-nginx" ]]; then
    alb-gh-release-nginx
  fi

  if [[ "$1" == "test-alb-go" ]]; then
    alb-gh-test-alb-go
  fi
  if [[ "$1" == "test-alb-nginx" ]]; then
    alb-gh-test-alb-nginx
  fi
  if [[ "$1" == "build-alb" ]]; then
    alb-gh-build-alb
  fi
  if [[ "$1" == "release-alb" ]]; then
    alb-gh-release-alb
  fi
fi
