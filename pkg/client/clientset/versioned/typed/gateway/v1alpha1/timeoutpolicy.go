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

// Code generated by client-gen. DO NOT EDIT.

package v1alpha1

import (
	"context"
	"time"

	v1alpha1 "alauda.io/alb2/pkg/apis/alauda/gateway/v1alpha1"
	scheme "alauda.io/alb2/pkg/client/clientset/versioned/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// TimeoutPoliciesGetter has a method to return a TimeoutPolicyInterface.
// A group's client should implement this interface.
type TimeoutPoliciesGetter interface {
	TimeoutPolicies(namespace string) TimeoutPolicyInterface
}

// TimeoutPolicyInterface has methods to work with TimeoutPolicy resources.
type TimeoutPolicyInterface interface {
	Create(ctx context.Context, timeoutPolicy *v1alpha1.TimeoutPolicy, opts v1.CreateOptions) (*v1alpha1.TimeoutPolicy, error)
	Update(ctx context.Context, timeoutPolicy *v1alpha1.TimeoutPolicy, opts v1.UpdateOptions) (*v1alpha1.TimeoutPolicy, error)
	UpdateStatus(ctx context.Context, timeoutPolicy *v1alpha1.TimeoutPolicy, opts v1.UpdateOptions) (*v1alpha1.TimeoutPolicy, error)
	Delete(ctx context.Context, name string, opts v1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error
	Get(ctx context.Context, name string, opts v1.GetOptions) (*v1alpha1.TimeoutPolicy, error)
	List(ctx context.Context, opts v1.ListOptions) (*v1alpha1.TimeoutPolicyList, error)
	Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.TimeoutPolicy, err error)
	TimeoutPolicyExpansion
}

// timeoutPolicies implements TimeoutPolicyInterface
type timeoutPolicies struct {
	client rest.Interface
	ns     string
}

// newTimeoutPolicies returns a TimeoutPolicies
func newTimeoutPolicies(c *GatewayV1alpha1Client, namespace string) *timeoutPolicies {
	return &timeoutPolicies{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the timeoutPolicy, and returns the corresponding timeoutPolicy object, and an error if there is any.
func (c *timeoutPolicies) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.TimeoutPolicy, err error) {
	result = &v1alpha1.TimeoutPolicy{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("timeoutpolicies").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of TimeoutPolicies that match those selectors.
func (c *timeoutPolicies) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.TimeoutPolicyList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1alpha1.TimeoutPolicyList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("timeoutpolicies").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested timeoutPolicies.
func (c *timeoutPolicies) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("timeoutpolicies").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

// Create takes the representation of a timeoutPolicy and creates it.  Returns the server's representation of the timeoutPolicy, and an error, if there is any.
func (c *timeoutPolicies) Create(ctx context.Context, timeoutPolicy *v1alpha1.TimeoutPolicy, opts v1.CreateOptions) (result *v1alpha1.TimeoutPolicy, err error) {
	result = &v1alpha1.TimeoutPolicy{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("timeoutpolicies").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(timeoutPolicy).
		Do(ctx).
		Into(result)
	return
}

// Update takes the representation of a timeoutPolicy and updates it. Returns the server's representation of the timeoutPolicy, and an error, if there is any.
func (c *timeoutPolicies) Update(ctx context.Context, timeoutPolicy *v1alpha1.TimeoutPolicy, opts v1.UpdateOptions) (result *v1alpha1.TimeoutPolicy, err error) {
	result = &v1alpha1.TimeoutPolicy{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("timeoutpolicies").
		Name(timeoutPolicy.Name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(timeoutPolicy).
		Do(ctx).
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *timeoutPolicies) UpdateStatus(ctx context.Context, timeoutPolicy *v1alpha1.TimeoutPolicy, opts v1.UpdateOptions) (result *v1alpha1.TimeoutPolicy, err error) {
	result = &v1alpha1.TimeoutPolicy{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("timeoutpolicies").
		Name(timeoutPolicy.Name).
		SubResource("status").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(timeoutPolicy).
		Do(ctx).
		Into(result)
	return
}

// Delete takes name of the timeoutPolicy and deletes it. Returns an error if one occurs.
func (c *timeoutPolicies) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("timeoutpolicies").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *timeoutPolicies) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Namespace(c.ns).
		Resource("timeoutpolicies").
		VersionedParams(&listOpts, scheme.ParameterCodec).
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

// Patch applies the patch and returns the patched timeoutPolicy.
func (c *timeoutPolicies) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.TimeoutPolicy, err error) {
	result = &v1alpha1.TimeoutPolicy{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("timeoutpolicies").
		Name(name).
		SubResource(subresources...).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)
	return
}