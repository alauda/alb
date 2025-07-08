#! /bin/bash

# [4.0,) 的升级alb chart的方式
# use kubecofnig in global cluster
function alb-4-upgrade() (
  # 1. 更新global集群上mplugin
  # 2. 等到对应版本的mconfig生成
  # 3. 更新global集群上的minfo
  local version=$1
  kubectl patch mplugin alb --type='json' -p "[{\"op\": \"replace\", \"path\": \"/spec/appReleases/0/chartVersions/0/version\", \"value\":\"$version\"}]"
  kubectl get mplugin alb -o yaml | grep version
  while true; do
    echo "wait $version mconfig"
    if kubectl get mconfig -n cpaas-system | grep alb-$version; then
      break
    fi
    sleep 1
  done

  kubectl get minfo -n cpaas-system | grep alb | awk '{print $1}' | xargs -I{} bash -c "kubectl patch --type='json' minfo {} -n cpaas-system -p  '[{\"op\": \"replace\", \"path\": \"/spec/version\",\"value\":\"$version\"}]' "
  return
)
