package client

import (
	"context"

	albv1 "alauda.io/alb2/pkg/apis/alauda/v1"
	albv2 "alauda.io/alb2/pkg/apis/alauda/v2beta1"
	appsv1 "k8s.io/api/apps/v1"
	coov1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	gv1a2t "sigs.k8s.io/gateway-api/apis/v1alpha2"
	gv1b1t "sigs.k8s.io/gateway-api/apis/v1beta1"
)

// init controller runtime client

func InitScheme(scheme *runtime.Scheme) *runtime.Scheme {
	_ = albv2.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)
	_ = albv1.AddToScheme(scheme)
	_ = rbacv1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)
	_ = netv1.AddToScheme(scheme)
	_ = gv1b1t.AddToScheme(scheme)
	_ = gv1a2t.AddToScheme(scheme)
	_ = coov1.AddToScheme(scheme)

	return scheme
}

func GetDirectlyClient(ctx context.Context, cfg *rest.Config, scheme *runtime.Scheme) (client.Client, error) {
	return client.New(cfg, client.Options{Scheme: scheme})
}

func GetClient(ctx context.Context, cfg *rest.Config, scheme *runtime.Scheme) (client.Client, error) {
	mapper, err := apiutil.NewDynamicRESTMapper(cfg)
	if err != nil {
		return nil, err
	}
	c, err := client.New(cfg, client.Options{Scheme: scheme, Mapper: mapper})
	if err != nil {
		return nil, err
	}
	cache, err := cache.New(cfg, cache.Options{Scheme: scheme, Mapper: mapper})
	go cache.Start(ctx)
	cache.WaitForCacheSync(ctx)
	if err != nil {
		return nil, err
	}
	return client.NewDelegatingClient(client.NewDelegatingClientInput{
		CacheReader: cache,
		Client:      c,
	})
}
