#!/bin/bash
component=$1
version=$2
docker pull index.alauda.cn/alaudaorg/$component:$version
docker tag index.alauda.cn/alaudaorg/$component:$version index.alauda.cn/claas/$component:$version
docker tag index.alauda.cn/alaudaorg/$component:$version index.alauda.io/claas/$component:$version
docker push index.alauda.cn/claas/$component:$version
#docker push index.alauda.io/claas/$component:$version
