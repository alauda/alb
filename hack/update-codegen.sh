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

CFD="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
ALB2_ROOT=${CFD}/..
# use `go get k8s.io/code-generator@v0.19.11-rc.0` to install correspond code_generator first
CODEGEN_PKG=$GOPATH/pkg/mod/k8s.io/code-generator@v0.19.11-rc.0



# generate the code with:
# --output-base    because this script should also be able to run inside the vendor dir of
#                  k8s.io/kubernetes. The output-base is needed for the generators to output into the vendor dir
#                  instead of the $GOPATH directly. For normal projects this can be dropped.
${CODEGEN_PKG}/generate-groups.sh "deepcopy,client,lister,informer" \
  alauda.io/alb2/pkg/client alauda.io/alb2/pkg/apis \
  "alauda:gateway/v1alpha1  alauda:v1"  \
  --go-header-file ${ALB2_ROOT}/hack/boilerplate.go.txt \
  --output-base "${ALB2_ROOT}/code_gen"

cp -r ${ALB2_ROOT}/code_gen/alauda.io/alb2/pkg/* ${ALB2_ROOT}/pkg
rm -rf ${ALB2_ROOT}/code_gen 

# To use your own boilerplate text append:
#   --go-header-file ${SCRIPT_ROOT}/hack/custom-boilerplate.go.txt
