#!/bin/bash

# Copyright 2017 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -o errexit
set -o nounset
set -o pipefail
set -x

CFD="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
ALB2_ROOT=${CFD}/..
CODEGEN_PKG=$GOPATH/pkg/mod/k8s.io/code-generator@v0.24.4-rc.0

rm -rf ${ALB2_ROOT}/code_gen || true
mkdir ${ALB2_ROOT}/code_gen

#  <generators> <output-package> <apis-package> <groups-versions> ...

${CODEGEN_PKG}/generate-groups.sh "client,lister,informer" \
  alauda.io/alb2/pkg/client alauda.io/alb2/pkg/apis \
  "alauda:gateway/v1alpha1 alauda:v1,v2beta1" \
  --go-header-file ${ALB2_ROOT}/scripts/boilerplate.go.txt \
  --output-base "${ALB2_ROOT}/code_gen"

${CODEGEN_PKG}/generate-groups.sh "deepcopy" \
  alauda.io/alb2/pkg/client alauda.io/alb2/pkg/apis \
  "alauda:v2beta1" \
  --go-header-file ${ALB2_ROOT}/scripts/boilerplate.go.txt \
  --output-base "${ALB2_ROOT}/code_gen"

tree ${ALB2_ROOT}/code_gen
cp -r ${ALB2_ROOT}/code_gen/alauda.io/alb2/pkg/* ${ALB2_ROOT}/pkg
rm -rf ${ALB2_ROOT}/code_gen

# we should use --plural-exceptions "ALB2:alaudaloadbalancer2,but we could not. is too late. i just give up.
find ./pkg/client -name '*.go' | xargs -i{} sed -i 's/Resource("alb2s")/Resource("alaudaloadbalancer2")/g' {}
