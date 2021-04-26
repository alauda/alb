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

package v1

import (
	"context"
	"time"

	v1 "alauda.io/alb2/pkg/apis/alauda/v1"
	scheme "alauda.io/alb2/pkg/client/clientset/versioned/scheme"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// ALB2sGetter has a method to return a ALB2Interface.
// A group's client should implement this interface.
type ALB2sGetter interface {
	ALB2s(namespace string) ALB2Interface
}

// ALB2Interface has methods to work with ALB2 resources.
type ALB2Interface interface {
	Create(ctx context.Context, aLB2 *v1.ALB2, opts metav1.CreateOptions) (*v1.ALB2, error)
	Update(ctx context.Context, aLB2 *v1.ALB2, opts metav1.UpdateOptions) (*v1.ALB2, error)
	UpdateStatus(ctx context.Context, aLB2 *v1.ALB2, opts metav1.UpdateOptions) (*v1.ALB2, error)
	Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Get(ctx context.Context, name string, opts metav1.GetOptions) (*v1.ALB2, error)
	List(ctx context.Context, opts metav1.ListOptions) (*v1.ALB2List, error)
	Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *v1.ALB2, err error)
	ALB2Expansion
}

// aLB2s implements ALB2Interface
type aLB2s struct {
	client rest.Interface
	ns     string
}

// newALB2s returns a ALB2s
func newALB2s(c *CrdV1Client, namespace string) *aLB2s {
	return &aLB2s{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the aLB2, and returns the corresponding aLB2 object, and an error if there is any.
func (c *aLB2s) Get(ctx context.Context, name string, options metav1.GetOptions) (result *v1.ALB2, err error) {
	result = &v1.ALB2{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("alaudaloadbalancer2").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of ALB2s that match those selectors.
func (c *aLB2s) List(ctx context.Context, opts metav1.ListOptions) (result *v1.ALB2List, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1.ALB2List{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("alaudaloadbalancer2").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested aLB2s.
func (c *aLB2s) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("alaudaloadbalancer2").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

// Create takes the representation of a aLB2 and creates it.  Returns the server's representation of the aLB2, and an error, if there is any.
func (c *aLB2s) Create(ctx context.Context, aLB2 *v1.ALB2, opts metav1.CreateOptions) (result *v1.ALB2, err error) {
	result = &v1.ALB2{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("alaudaloadbalancer2").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(aLB2).
		Do(ctx).
		Into(result)
	return
}

// Update takes the representation of a aLB2 and updates it. Returns the server's representation of the aLB2, and an error, if there is any.
func (c *aLB2s) Update(ctx context.Context, aLB2 *v1.ALB2, opts metav1.UpdateOptions) (result *v1.ALB2, err error) {
	result = &v1.ALB2{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("alaudaloadbalancer2").
		Name(aLB2.Name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(aLB2).
		Do(ctx).
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *aLB2s) UpdateStatus(ctx context.Context, aLB2 *v1.ALB2, opts metav1.UpdateOptions) (result *v1.ALB2, err error) {
	result = &v1.ALB2{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("alaudaloadbalancer2").
		Name(aLB2.Name).
		SubResource("status").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(aLB2).
		Do(ctx).
		Into(result)
	return
}

// Delete takes name of the aLB2 and deletes it. Returns an error if one occurs.
func (c *aLB2s) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("alaudaloadbalancer2").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *aLB2s) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Namespace(c.ns).
		Resource("alaudaloadbalancer2").
		VersionedParams(&listOpts, scheme.ParameterCodec).
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

// Patch applies the patch and returns the patched aLB2.
func (c *aLB2s) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *v1.ALB2, err error) {
	result = &v1.ALB2{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("alaudaloadbalancer2").
		Name(name).
		SubResource(subresources...).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)
	return
}
