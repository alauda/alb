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

package v1alpha1

import (
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PolicyTargetReference) DeepCopyInto(out *PolicyTargetReference) {
	*out = *in
	if in.SectionIndex != nil {
		in, out := &in.SectionIndex, &out.SectionIndex
		*out = new(uint)
		**out = **in
	}
	if in.SectionName != nil {
		in, out := &in.SectionName, &out.SectionName
		*out = new(string)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PolicyTargetReference.
func (in *PolicyTargetReference) DeepCopy() *PolicyTargetReference {
	if in == nil {
		return nil
	}
	out := new(PolicyTargetReference)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TimeoutPolicy) DeepCopyInto(out *TimeoutPolicy) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TimeoutPolicy.
func (in *TimeoutPolicy) DeepCopy() *TimeoutPolicy {
	if in == nil {
		return nil
	}
	out := new(TimeoutPolicy)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *TimeoutPolicy) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TimeoutPolicyConfig) DeepCopyInto(out *TimeoutPolicyConfig) {
	*out = *in
	if in.ProxyConnectTimeoutMs != nil {
		in, out := &in.ProxyConnectTimeoutMs, &out.ProxyConnectTimeoutMs
		*out = new(uint)
		**out = **in
	}
	if in.ProxySendTimeoutMs != nil {
		in, out := &in.ProxySendTimeoutMs, &out.ProxySendTimeoutMs
		*out = new(uint)
		**out = **in
	}
	if in.ProxyReadTimeoutMs != nil {
		in, out := &in.ProxyReadTimeoutMs, &out.ProxyReadTimeoutMs
		*out = new(uint)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TimeoutPolicyConfig.
func (in *TimeoutPolicyConfig) DeepCopy() *TimeoutPolicyConfig {
	if in == nil {
		return nil
	}
	out := new(TimeoutPolicyConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TimeoutPolicyList) DeepCopyInto(out *TimeoutPolicyList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]TimeoutPolicy, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TimeoutPolicyList.
func (in *TimeoutPolicyList) DeepCopy() *TimeoutPolicyList {
	if in == nil {
		return nil
	}
	out := new(TimeoutPolicyList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *TimeoutPolicyList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TimeoutPolicySpec) DeepCopyInto(out *TimeoutPolicySpec) {
	*out = *in
	in.TargetRef.DeepCopyInto(&out.TargetRef)
	if in.Override != nil {
		in, out := &in.Override, &out.Override
		*out = new(TimeoutPolicyConfig)
		(*in).DeepCopyInto(*out)
	}
	if in.Default != nil {
		in, out := &in.Default, &out.Default
		*out = new(TimeoutPolicyConfig)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TimeoutPolicySpec.
func (in *TimeoutPolicySpec) DeepCopy() *TimeoutPolicySpec {
	if in == nil {
		return nil
	}
	out := new(TimeoutPolicySpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TimeoutPolicyStatus) DeepCopyInto(out *TimeoutPolicyStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]v1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TimeoutPolicyStatus.
func (in *TimeoutPolicyStatus) DeepCopy() *TimeoutPolicyStatus {
	if in == nil {
		return nil
	}
	out := new(TimeoutPolicyStatus)
	in.DeepCopyInto(out)
	return out
}
