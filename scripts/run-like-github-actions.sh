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
  ls
  cat ./luacov.summary
)

function alb-gh-build-alb() (
  env
  local chart_ver=$(alb-github-gen-version)
  echo "version $CURRENT_VERSION this ver $chart_ver is_release $RELEASE_ME"

  local alb_nginx_base="$IMAGE_REPO/alb-nginx-base:$(alb-gh-get-nginx-ver)"
  local go_build_base="docker.io/library/golang:$(alb-gh-get-gobuild-ver)"
  #  build images

  local platform=${MATRIX_PLATFORM:-"linux/amd64"}
  echo "platform $platform"
  docker buildx build \
    --network=host \
    --platform $platform \
    -t $IMAGE_REPO/alb:$chart_ver \
    --build-arg GO_BUILD_BASE=$go_build_base \
    --build-arg ALB_ONLINE=true \
    --build-arg OPENRESTY_BASE=$alb_nginx_base \
    -o type=docker \
    -f ./Dockerfile \
    .
  docker images
  docker image inspect $IMAGE_REPO/alb:$chart_ver
  local suffix=$(__normal_platform)
  docker save $IMAGE_REPO/alb:$chart_ver >./alb-$suffix.tar
  ls
  return
)

function alb-gh-gen-chart-artifact() (
  rm -rf .cr-release-packages
  mkdir -p .cr-release-packages
  local chart_ver=${1-$(alb-github-gen-version)}
  local chart=$(alb-build-github-chart $IMAGE_REPO $chart_ver ./deploy/chart/alb .cr-release-packages/)
  echo "tree .cr-release-packages"
  tree .cr-release-packages
  echo "chart_ver $chart_ver chart $chart"
  cp $chart alauda-alb2.tgz
)

function alb-gh-release-alb() (
  set -x
  echo "$DOCKER_PASSWORD" | docker login -u "$DOCKER_USERNAME" --password-stdin
  echo "in release"

  tree ./.cr-release-packages
  # the loaded image tag are $(alb-github-gen-version)
  docker load <./alb-linux-amd64.tar # TODO
  docker images

  if [[ "$RELEASE_TYPE" == "pre-release" ]]; then
    local version=$(alb-github-gen-version)
    docker tag $IMAGE_REPO/alb:$version $IMAGE_REPO/alb:v0.0.0
    docker image inspect $IMAGE_REPO/alb:v0.0.0
    alb-gh-gen-chart-artifact v0.0.0
    docker push $IMAGE_REPO/alb:v0.0.0
    # update release
    git tag --delete alauda-alb2-v0.0.0 || true
    git push origin --delete alauda-alb2-v0.0.0 || true
    git tag alauda-alb2-v0.0.0
    git push origin alauda-alb2-v0.0.0
    git tag | cat
    gh release delete alauda-alb2-v0.0.0 || true
    gh release create alauda-alb2-v0.0.0 --prerelease ./.cr-release-packages/alauda-alb2-v0.0.0.tgz
    return
  fi

  if [[ "$RELEASE_TYPE" == "release" ]]; then
    local version=$(alb-github-gen-version)
    docker image inspect $IMAGE_REPO/alb:$version
    docker push $IMAGE_REPO/alb:$version
    git fetch --all # https://github.com/JamesIves/github-pages-deploy-action/issues/74
    # yes. we allow to re-release
    git tag --delete alauda-alb2-$version || true
    git push origin --delete alauda-alb2-$version || true
    git tag alauda-alb2-$version
    git push origin alauda-alb2-$version
    git tag | cat
    gh release delete alauda-alb2-$version || true
    .github/cr.sh --owner "alauda" --repo "alb" --charts-dir "./deploy/chart/alb" --skip-packaging "true" --pages-branch "gh-pages"
  fi
  return
)

function alb-gh-build-nginx() (
  local ver=$(alb-gh-get-nginx-ver)
  local RESTY_PCRE_VERSION=$(cat ./template/Dockerfile.openresty | grep RESTY_PCRE_VERSION= | awk -F = '{print $2}' | tr -d '"')
  local RESTY_PCRE_BASE="https://downloads.sourceforge.net/project/pcre/pcre/$RESTY_PCRE_VERSION/pcre-$RESTY_PCRE_VERSION.tar.gz"
  local MODSECURITY_NGINX_BASE="https://codeload.github.com/owasp-modsecurity/ModSecurity-nginx/zip/refs/heads/master"
  local MODSECURITY_BASE="https://github.com/owasp-modsecurity/ModSecurity/releases/download/v3.0.13/modsecurity-v3.0.13.tar.gz"
  local resty_base="docker.io/library/alpine"

  local platform=${MATRIX_PLATFORM:-"linux/amd64"}
  echo "platform $platform"
  docker buildx build \
    --network=host \
    --platform $platform \
    --no-cache \
    --network=host \
    -t $IMAGE_REPO/alb-nginx-base:$ver \
    --build-arg RESTY_IMAGE_BASE=$resty_base \
    --build-arg RESTY_PCRE_BASE=$RESTY_PCRE_BASE \
    --build-arg MODSECURITY_NGINX_BASE=$MODSECURITY_NGINX_BASE \
    --build-arg MODSECURITY_BASE=$MODSECURITY_BASE \
    -o type=docker \
    -f ./template/Dockerfile.openresty \
    ./
  docker images
  docker image inspect $IMAGE_REPO/alb-nginx-base:$ver
  local suffix=$(__normal_platform)
  echo "export ./alb-nginx-base-$suffix.tar"
  docker save $IMAGE_REPO/alb-nginx-base:$ver >./alb-nginx-base-$suffix.tar
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
  cat ./Dockerfile | grep OPENRESTY_BASE | grep alb-nginx | awk -F : '{print $2}' | xargs
)

function alb-gh-get-gobuild-ver() (
  cat ./Dockerfile | grep GO_BUILD_BASE= | awk -F : '{print $2}'
)

function alb-github-gen-version() {
  if [[ "$RELEASE_TYPE" == "release" ]]; then
    echo v$CURRENT_VERSION
    return
  fi
  echo "v$CURRENT_VERSION-$branch.$GITHUB_RUN_NUMBER.$GITHUB_RUN_ATTEMPT"
}

function __normal_platform() {
  echo "$MATRIX_PLATFORM" | sed 's|/|-|g'
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
  if [[ "$1" == "gen-chart-artifact" ]]; then
    alb-gh-gen-chart-artifact
  fi
fi
