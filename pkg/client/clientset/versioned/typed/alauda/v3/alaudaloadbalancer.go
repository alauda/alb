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

package v3

import (
	v3 "alb2/pkg/apis/alauda/v3"
	scheme "alb2/pkg/client/clientset/versioned/scheme"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// AlaudaLoadBalancersGetter has a method to return a AlaudaLoadBalancerInterface.
// A group's client should implement this interface.
type AlaudaLoadBalancersGetter interface {
	AlaudaLoadBalancers() AlaudaLoadBalancerInterface
}

// AlaudaLoadBalancerInterface has methods to work with AlaudaLoadBalancer resources.
type AlaudaLoadBalancerInterface interface {
	Create(*v3.AlaudaLoadBalancer) (*v3.AlaudaLoadBalancer, error)
	Update(*v3.AlaudaLoadBalancer) (*v3.AlaudaLoadBalancer, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*v3.AlaudaLoadBalancer, error)
	List(opts v1.ListOptions) (*v3.AlaudaLoadBalancerList, error)
	Watch(opts v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v3.AlaudaLoadBalancer, err error)
	AlaudaLoadBalancerExpansion
}

// alaudaLoadBalancers implements AlaudaLoadBalancerInterface
type alaudaLoadBalancers struct {
	client rest.Interface
}

// newAlaudaLoadBalancers returns a AlaudaLoadBalancers
func newAlaudaLoadBalancers(c *CrdV3Client) *alaudaLoadBalancers {
	return &alaudaLoadBalancers{
		client: c.RESTClient(),
	}
}

// Get takes name of the alaudaLoadBalancer, and returns the corresponding alaudaLoadBalancer object, and an error if there is any.
func (c *alaudaLoadBalancers) Get(name string, options v1.GetOptions) (result *v3.AlaudaLoadBalancer, err error) {
	result = &v3.AlaudaLoadBalancer{}
	err = c.client.Get().
		Resource("alaudaloadbalancers").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of AlaudaLoadBalancers that match those selectors.
func (c *alaudaLoadBalancers) List(opts v1.ListOptions) (result *v3.AlaudaLoadBalancerList, err error) {
	result = &v3.AlaudaLoadBalancerList{}
	err = c.client.Get().
		Resource("alaudaloadbalancers").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested alaudaLoadBalancers.
func (c *alaudaLoadBalancers) Watch(opts v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Resource("alaudaloadbalancers").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Create takes the representation of a alaudaLoadBalancer and creates it.  Returns the server's representation of the alaudaLoadBalancer, and an error, if there is any.
func (c *alaudaLoadBalancers) Create(alaudaLoadBalancer *v3.AlaudaLoadBalancer) (result *v3.AlaudaLoadBalancer, err error) {
	result = &v3.AlaudaLoadBalancer{}
	err = c.client.Post().
		Resource("alaudaloadbalancers").
		Body(alaudaLoadBalancer).
		Do().
		Into(result)
	return
}

// Update takes the representation of a alaudaLoadBalancer and updates it. Returns the server's representation of the alaudaLoadBalancer, and an error, if there is any.
func (c *alaudaLoadBalancers) Update(alaudaLoadBalancer *v3.AlaudaLoadBalancer) (result *v3.AlaudaLoadBalancer, err error) {
	result = &v3.AlaudaLoadBalancer{}
	err = c.client.Put().
		Resource("alaudaloadbalancers").
		Name(alaudaLoadBalancer.Name).
		Body(alaudaLoadBalancer).
		Do().
		Into(result)
	return
}

// Delete takes name of the alaudaLoadBalancer and deletes it. Returns an error if one occurs.
func (c *alaudaLoadBalancers) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Resource("alaudaloadbalancers").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *alaudaLoadBalancers) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	return c.client.Delete().
		Resource("alaudaloadbalancers").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched alaudaLoadBalancer.
func (c *alaudaLoadBalancers) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v3.AlaudaLoadBalancer, err error) {
	result = &v3.AlaudaLoadBalancer{}
	err = c.client.Patch(pt).
		Resource("alaudaloadbalancers").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
