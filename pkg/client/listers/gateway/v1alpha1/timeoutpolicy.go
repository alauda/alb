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

// Code generated by lister-gen. DO NOT EDIT.

package v1alpha1

import (
	v1alpha1 "alauda.io/alb2/pkg/apis/alauda/gateway/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// TimeoutPolicyLister helps list TimeoutPolicies.
// All objects returned here must be treated as read-only.
type TimeoutPolicyLister interface {
	// List lists all TimeoutPolicies in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.TimeoutPolicy, err error)
	// TimeoutPolicies returns an object that can list and get TimeoutPolicies.
	TimeoutPolicies(namespace string) TimeoutPolicyNamespaceLister
	TimeoutPolicyListerExpansion
}

// timeoutPolicyLister implements the TimeoutPolicyLister interface.
type timeoutPolicyLister struct {
	indexer cache.Indexer
}

// NewTimeoutPolicyLister returns a new TimeoutPolicyLister.
func NewTimeoutPolicyLister(indexer cache.Indexer) TimeoutPolicyLister {
	return &timeoutPolicyLister{indexer: indexer}
}

// List lists all TimeoutPolicies in the indexer.
func (s *timeoutPolicyLister) List(selector labels.Selector) (ret []*v1alpha1.TimeoutPolicy, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.TimeoutPolicy))
	})
	return ret, err
}

// TimeoutPolicies returns an object that can list and get TimeoutPolicies.
func (s *timeoutPolicyLister) TimeoutPolicies(namespace string) TimeoutPolicyNamespaceLister {
	return timeoutPolicyNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// TimeoutPolicyNamespaceLister helps list and get TimeoutPolicies.
// All objects returned here must be treated as read-only.
type TimeoutPolicyNamespaceLister interface {
	// List lists all TimeoutPolicies in the indexer for a given namespace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.TimeoutPolicy, err error)
	// Get retrieves the TimeoutPolicy from the indexer for a given namespace and name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*v1alpha1.TimeoutPolicy, error)
	TimeoutPolicyNamespaceListerExpansion
}

// timeoutPolicyNamespaceLister implements the TimeoutPolicyNamespaceLister
// interface.
type timeoutPolicyNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all TimeoutPolicies in the indexer for a given namespace.
func (s timeoutPolicyNamespaceLister) List(selector labels.Selector) (ret []*v1alpha1.TimeoutPolicy, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.TimeoutPolicy))
	})
	return ret, err
}

// Get retrieves the TimeoutPolicy from the indexer for a given namespace and name.
func (s timeoutPolicyNamespaceLister) Get(name string) (*v1alpha1.TimeoutPolicy, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1alpha1.Resource("timeoutpolicy"), name)
	}
	return obj.(*v1alpha1.TimeoutPolicy), nil
}
