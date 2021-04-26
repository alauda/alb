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

// Code generated by informer-gen. DO NOT EDIT.

package v1

import (
	"context"
	time "time"

	alaudav1 "alauda.io/alb2/pkg/apis/alauda/v1"
	versioned "alauda.io/alb2/pkg/client/clientset/versioned"
	internalinterfaces "alauda.io/alb2/pkg/client/informers/externalversions/internalinterfaces"
	v1 "alauda.io/alb2/pkg/client/listers/alauda/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
)

// RuleInformer provides access to a shared informer and lister for
// Rules.
type RuleInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1.RuleLister
}

type ruleInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
	namespace        string
}

// NewRuleInformer constructs a new informer for Rule type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewRuleInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredRuleInformer(client, namespace, resyncPeriod, indexers, nil)
}

// NewFilteredRuleInformer constructs a new informer for Rule type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredRuleInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.CrdV1().Rules(namespace).List(context.TODO(), options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.CrdV1().Rules(namespace).Watch(context.TODO(), options)
			},
		},
		&alaudav1.Rule{},
		resyncPeriod,
		indexers,
	)
}

func (f *ruleInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredRuleInformer(client, f.namespace, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *ruleInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&alaudav1.Rule{}, f.defaultInformer)
}

func (f *ruleInformer) Lister() v1.RuleLister {
	return v1.NewRuleLister(f.Informer().GetIndexer())
}
