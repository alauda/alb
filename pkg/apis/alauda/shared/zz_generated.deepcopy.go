//go:build !ignore_autogenerated
// +build !ignore_autogenerated

/*
Copyright The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by cursor-bin.AppImage. DO NOT EDIT.

package shared

import (
	authtypes "alauda.io/alb2/pkg/controller/ext/auth/types"
	types "alauda.io/alb2/pkg/controller/ext/otel/types"
	timeouttypes "alauda.io/alb2/pkg/controller/ext/timeout/types"
	waftypes "alauda.io/alb2/pkg/controller/ext/waf/types"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SharedCr) DeepCopyInto(out *SharedCr) {
	*out = *in
	if in.Otel != nil {
		in, out := &in.Otel, &out.Otel
		*out = new(types.OtelCrConf)
		(*in).DeepCopyInto(*out)
	}
	if in.ModeSecurity != nil {
		in, out := &in.ModeSecurity, &out.ModeSecurity
		*out = new(waftypes.WafCrConf)
		**out = **in
	}
	if in.Auth != nil {
		in, out := &in.Auth, &out.Auth
		*out = new(authtypes.AuthCr)
		(*in).DeepCopyInto(*out)
	}
	if in.Timeout != nil {
		in, out := &in.Timeout, &out.Timeout
		*out = new(timeouttypes.TimeoutCr)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SharedCr.
func (in *SharedCr) DeepCopy() *SharedCr {
	if in == nil {
		return nil
	}
	out := new(SharedCr)
	in.DeepCopyInto(out)
	return out
}
