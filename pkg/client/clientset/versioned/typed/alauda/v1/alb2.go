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
	v1 "alb2/pkg/apis/alauda/v1"
	scheme "alb2/pkg/client/clientset/versioned/scheme"

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
	Create(*v1.ALB2) (*v1.ALB2, error)
	Update(*v1.ALB2) (*v1.ALB2, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteCollection(options *metav1.DeleteOptions, listOptions metav1.ListOptions) error
	Get(name string, options metav1.GetOptions) (*v1.ALB2, error)
	List(opts metav1.ListOptions) (*v1.ALB2List, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.ALB2, err error)
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
func (c *aLB2s) Get(name string, options metav1.GetOptions) (result *v1.ALB2, err error) {
	result = &v1.ALB2{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("alaudaloadbalancer2").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of ALB2s that match those selectors.
func (c *aLB2s) List(opts metav1.ListOptions) (result *v1.ALB2List, err error) {
	result = &v1.ALB2List{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("alaudaloadbalancer2").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested aLB2s.
func (c *aLB2s) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("alaudaloadbalancer2").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Create takes the representation of a aLB2 and creates it.  Returns the server's representation of the aLB2, and an error, if there is any.
func (c *aLB2s) Create(aLB2 *v1.ALB2) (result *v1.ALB2, err error) {
	result = &v1.ALB2{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("alaudaloadbalancer2").
		Body(aLB2).
		Do().
		Into(result)
	return
}

// Update takes the representation of a aLB2 and updates it. Returns the server's representation of the aLB2, and an error, if there is any.
func (c *aLB2s) Update(aLB2 *v1.ALB2) (result *v1.ALB2, err error) {
	result = &v1.ALB2{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("alaudaloadbalancer2").
		Name(aLB2.Name).
		Body(aLB2).
		Do().
		Into(result)
	return
}

// Delete takes name of the aLB2 and deletes it. Returns an error if one occurs.
func (c *aLB2s) Delete(name string, options *metav1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("alaudaloadbalancer2").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *aLB2s) DeleteCollection(options *metav1.DeleteOptions, listOptions metav1.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("alaudaloadbalancer2").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched aLB2.
func (c *aLB2s) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.ALB2, err error) {
	result = &v1.ALB2{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("alaudaloadbalancer2").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
