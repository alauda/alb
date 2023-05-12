package resourceclient

import (
	"context"

	v1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gv1b1t "sigs.k8s.io/gateway-api/apis/v1beta1"
)

type ALB2ResourceClient struct {
	client.Client
}

// TODO use ctx from parent
func NewALB2ResourceClient(client client.Client) *ALB2ResourceClient {
	return &ALB2ResourceClient{Client: client}
}

func (c *ALB2ResourceClient) CreateOrUpdateConfigmap(cm *v1.ConfigMap) error {
	currentDeployment := &v1.ConfigMap{}
	err := c.Get(context.Background(), types.NamespacedName{
		Namespace: cm.GetNamespace(),
		Name:      cm.GetName(),
	}, currentDeployment)

	if err != nil {
		if errors.IsNotFound(err) {
			return c.Create(context.Background(), cm)
		}
		return err
	}
	cm.ResourceVersion = currentDeployment.GetResourceVersion()
	return c.Update(context.Background(), cm)
}

func (c *ALB2ResourceClient) CreateOrUpdateService(svc *v1.Service) error {
	currentDeployment := &v1.Service{}
	err := c.Get(context.Background(), types.NamespacedName{
		Namespace: svc.GetNamespace(),
		Name:      svc.GetName(),
	}, currentDeployment)

	if err != nil {
		if errors.IsNotFound(err) {
			return c.Create(context.Background(), svc)
		}
		return err
	}
	svc.ResourceVersion = currentDeployment.GetResourceVersion()
	return c.Update(context.Background(), svc)
}

func (c *ALB2ResourceClient) CreateOrUpdate(ctx context.Context, update bool, obj client.Object) error {
	if update {
		return c.Update(ctx, obj)
	}
	return c.Create(ctx, obj)
}

func (c *ALB2ResourceClient) CreateOrUpdateIngressClass(ic *netv1.IngressClass) error {
	currentIngressClass := &netv1.IngressClass{}
	err := c.Get(context.Background(), types.NamespacedName{
		Namespace: ic.GetNamespace(),
		Name:      ic.GetName(),
	}, currentIngressClass)

	if err != nil {
		if errors.IsNotFound(err) {
			return c.Create(context.Background(), ic)
		}
		return err
	}
	ic.ResourceVersion = currentIngressClass.GetResourceVersion()
	return c.Update(context.Background(), ic)
}

func (c *ALB2ResourceClient) CreateOrUpdateGatewayClass(gc *gv1b1t.GatewayClass) error {
	currentGatewayClass := &netv1.IngressClass{}
	err := c.Get(context.Background(), types.NamespacedName{
		Namespace: gc.GetNamespace(),
		Name:      gc.GetName(),
	}, currentGatewayClass)

	if err != nil {
		if errors.IsNotFound(err) {
			return c.Create(context.Background(), gc)
		}
		return err
	}
	gc.ResourceVersion = currentGatewayClass.GetResourceVersion()
	return c.Update(context.Background(), gc)
}
