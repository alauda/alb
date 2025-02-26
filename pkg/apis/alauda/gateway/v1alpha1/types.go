package v1alpha1

// +kubebuilder:validation:Optional

import (
	timeout_t "alauda.io/alb2/pkg/controller/ext/timeout/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	TimeoutPolicyKind = "TimeoutPolicy"
)

// PolicyTargetReference identifies an API object to apply policy to.
type PolicyTargetReference struct {
	// Group is the group of the target resource.
	//
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	Group string `json:"group"`

	// Kind is kind of the target resource.
	//
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	Kind string `json:"kind"`

	// Name is the name of the target resource.
	//
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	Name string `json:"name"`

	// Namespace is the namespace of the referent. When unspecified, the local
	// namespace is inferred. Even when policy targets a resource in a different
	// namespace, it may only apply to traffic originating from the same
	// namespace as the policy.
	//
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// SectionName is the index of a section within the target resource. When
	// unspecified, this targets the entire resource. When SectionIndex and SectionIndex Both exist, use SectionName.
	//
	// +optional
	SectionIndex *uint `json:"sectionIndex"`

	// SectionName is the name of a section within the target resource. When
	// unspecified, this targets the entire resource. In the following
	// resources, SectionName is interpreted as the following:
	// * Gateway: Listener Name
	// * Route: Rule Name
	// * Service: Port Name
	//
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +optional
	SectionName *string `json:"sectionName"`
}

type TimeoutPolicyConfig timeout_t.TimeoutCr

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:singular=timeoutpolicy,path=timeoutpolicies,shortName=timeout,scope=Namespaced
// +kubebuilder:subresource:status
type TimeoutPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TimeoutPolicySpec   `json:"spec"`
	Status TimeoutPolicyStatus `json:"status"`
}

type TimeoutPolicySpec struct {
	TargetRef PolicyTargetReference `json:"targetRef"`
	// Override defines policy configuration that should override policy
	// configuration attached below the targeted resource in the hierarchy.
	// +optional
	Override *TimeoutPolicyConfig `json:"override,omitempty"`

	// Default defines default policy configuration for the targeted resource.
	// +optional
	Default *TimeoutPolicyConfig `json:"default,omitempty"`
}

type TimeoutPolicyStatus struct {
	// Conditions describe the current conditions of the  TimeoutPolicy.
	//
	// +optional
	// +listType=map
	// +listMapKey=type
	// +kubebuilder:validation:MaxItems=8
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type TimeoutPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []TimeoutPolicy `json:"items"`
}
