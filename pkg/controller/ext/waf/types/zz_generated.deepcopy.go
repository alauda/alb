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

// Code generated by deepcopy-gen. DO NOT EDIT.

package types

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *WafConf) DeepCopyInto(out *WafConf) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WafConf.
func (in *WafConf) DeepCopy() *WafConf {
	if in == nil {
		return nil
	}
	out := new(WafConf)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *WafCrConf) DeepCopyInto(out *WafCrConf) {
	*out = *in
	out.WafConf = in.WafConf
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WafCrConf.
func (in *WafCrConf) DeepCopy() *WafCrConf {
	if in == nil {
		return nil
	}
	out := new(WafCrConf)
	in.DeepCopyInto(out)
	return out
}
